package campus

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxFileSize = 5 << 20

type Material struct {
	ID          int64     `json:"id"`
	ClassID     int64     `json:"class_id"`
	Title       string    `json:"title"`
	Filename    string    `json:"original_filename"`
	ByteSize    int64     `json:"byte_size"`
	Created     time.Time `json:"created_at"`
	Body        string    `json:"body_text,omitempty"`
	KnowledgeID int64     `json:"knowledge_entry_id,omitempty"`
	Storage     string    `json:"-"`
}
type Knowledge struct {
	ID         int64  `json:"id"`
	MaterialID int64  `json:"material_id"`
	ClassID    int64  `json:"class_id"`
	Body       string `json:"body_text"`
}

func (s *Server) listMaterials(w http.ResponseWriter, r *http.Request) {
	rows, e := s.DB.QueryContext(r.Context(), "SELECT id,class_id,title,original_filename,byte_size,created_at FROM materials WHERE class_id=? ORDER BY id DESC", currentUser(r).ClassID)
	if e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	defer rows.Close()
	out := []Material{}
	for rows.Next() {
		var m Material
		if e = rows.Scan(&m.ID, &m.ClassID, &m.Title, &m.Filename, &m.ByteSize, &m.Created); e != nil {
			problem(w, 503, "服务暂不可用")
			return
		}
		out = append(out, m)
	}
	if rows.Err() != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	writeJSON(w, 200, out)
}

// The tutorial explicitly separates fetch-by-ID from the class check.
// Never serialize or open the file until both steps have passed.
func (s *Server) ownedMaterial(r *http.Request) (Material, error) {
	var m Material
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id < 1 {
		return m, sql.ErrNoRows
	}
	e = s.DB.QueryRowContext(r.Context(), "SELECT m.id,m.class_id,m.title,m.original_filename,m.byte_size,m.created_at,m.storage_key,k.id,k.body_text FROM materials m JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id WHERE m.id=?", id).Scan(&m.ID, &m.ClassID, &m.Title, &m.Filename, &m.ByteSize, &m.Created, &m.Storage, &m.KnowledgeID, &m.Body)
	if e != nil {
		return m, e
	}
	if m.ClassID != currentUser(r).ClassID {
		logReason("cross-class material denied")
		return Material{}, sql.ErrNoRows
	}
	return m, nil
}
func resourceError(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		problem(w, 404, "资源不存在")
	} else {
		problem(w, 503, "服务暂不可用")
	}
}
func (s *Server) materialDetail(w http.ResponseWriter, r *http.Request) {
	m, e := s.ownedMaterial(r)
	if e != nil {
		resourceError(w, e)
		return
	}
	writeJSON(w, 200, m)
}
func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	m, e := s.ownedMaterial(r)
	if e != nil {
		resourceError(w, e)
		return
	}
	path, e := s.storagePath(m.Storage)
	if e != nil {
		problem(w, 500, "文件不可用")
		return
	}
	file, e := os.Open(path)
	if e != nil {
		problem(w, 404, "资源不存在")
		return
	}
	defer file.Close()
	stat, e := file.Stat()
	if e != nil {
		problem(w, 500, "文件不可用")
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": m.Filename}))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, m.Filename, stat.ModTime(), file)
}
func (s *Server) listKnowledge(w http.ResponseWriter, r *http.Request) {
	rows, e := s.DB.QueryContext(r.Context(), "SELECT id,material_id,class_id,body_text FROM knowledge_entries WHERE class_id=? ORDER BY id DESC", currentUser(r).ClassID)
	if e != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	defer rows.Close()
	out := []Knowledge{}
	for rows.Next() {
		var k Knowledge
		if e = rows.Scan(&k.ID, &k.MaterialID, &k.ClassID, &k.Body); e != nil {
			problem(w, 503, "服务暂不可用")
			return
		}
		out = append(out, k)
	}
	if rows.Err() != nil {
		problem(w, 503, "服务暂不可用")
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) knowledgeDetail(w http.ResponseWriter, r *http.Request) {
	var k Knowledge
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id < 1 {
		resourceError(w, sql.ErrNoRows)
		return
	}
	e = s.DB.QueryRowContext(r.Context(), "SELECT id,material_id,class_id,body_text FROM knowledge_entries WHERE id=?", id).Scan(&k.ID, &k.MaterialID, &k.ClassID, &k.Body)
	if e == nil && k.ClassID != currentUser(r).ClassID {
		logReason("cross-class knowledge denied")
		e = sql.ErrNoRows
	}
	if e != nil {
		resourceError(w, e)
		return
	}
	writeJSON(w, 200, k)
}
func validateText(filename string, data []byte) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".txt" && ext != ".md" {
		return "", "", errors.New("仅支持 txt/md 文件")
	}
	if len(data) > maxFileSize {
		return "", "", errors.New("文件超过 5 MiB")
	}
	if !utf8.Valid(data) {
		return "", "", errors.New("文件必须为 UTF-8")
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	if strings.TrimSpace(text) == "" {
		return "", "", errors.New("文件正文不能为空")
	}
	return ext, text, nil
}
func (s *Server) storagePath(key string) (string, error) {
	if filepath.IsAbs(key) || strings.HasPrefix(key, "/") || strings.ContainsAny(key, "\\:") {
		return "", errors.New("invalid storage key")
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid storage key")
	}
	return filepath.Join(s.Config.UploadDir, clean), nil
}
func (s *Server) saveFile(classID int64, ext string, data []byte) (string, string, error) {
	token, e := randomToken()
	if e != nil {
		return "", "", e
	}
	key := fmt.Sprintf("%d/%s%s", classID, token, ext)
	path, e := s.storagePath(key)
	if e != nil {
		return "", "", e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return "", "", e
	}
	if e = os.MkdirAll(filepath.Join(s.Config.UploadDir, ".tmp"), 0700); e != nil {
		return "", "", e
	}
	f, e := os.CreateTemp(filepath.Join(s.Config.UploadDir, ".tmp"), "upload-")
	if e != nil {
		return "", "", e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = s.inject("file-write"); e != nil {
		f.Close()
		return "", "", e
	}
	if _, e = f.Write(data); e != nil {
		f.Close()
		return "", "", e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return "", "", e
	}
	if e = f.Close(); e != nil {
		return "", "", e
	}
	if e = os.Rename(temp, path); e != nil {
		return "", "", e
	}
	return key, path, nil
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.Role != "teacher" {
		problem(w, 403, "学生无上传权限")
		return
	}
	if !validCSRF(r, u) {
		problem(w, 403, "请求校验失败")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	// MaxMemory exceeds total request bound: multipart cannot spill unchecked files
	// onto disk before validation. RemoveAll is defensive on parser failures.
	e := r.ParseMultipartForm(6 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if e != nil {
		var maxErr *http.MaxBytesError
		if errors.As(e, &maxErr) {
			problem(w, 413, "文件超过大小限制")
		} else {
			problem(w, 400, "需要 multipart 文件")
		}
		return
	}
	file, header, e := r.FormFile("file")
	if e != nil {
		problem(w, 400, "缺少文件")
		return
	}
	defer file.Close()
	data, e := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if e != nil {
		problem(w, 400, "无法读取文件")
		return
	}
	if len(data) > maxFileSize {
		problem(w, 413, "文件超过 5 MiB")
		return
	}
	ext, text, e := validateText(header.Filename, data)
	if e != nil {
		problem(w, 400, e.Error())
		return
	}
	filename := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = filename
	}
	if utf8.RuneCountInString(filename) > 255 || utf8.RuneCountInString(title) > 255 {
		problem(w, 400, "名称过长")
		return
	}
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	key, path, e := s.saveFile(u.ClassID, ext, data)
	if e != nil {
		problem(w, 500, "文件保存失败")
		return
	}
	keep := false
	defer func() {
		if !keep {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				s.unhealthy.Store(true)
			}
		}
	}()
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 500, "入库失败")
		return
	}
	defer tx.Rollback()
	result, e := tx.ExecContext(r.Context(), "INSERT INTO materials(class_id,uploader,title,original_filename,storage_key,byte_size) VALUES(?,?,?,?,?,?)", u.ClassID, u.ID, title, filename, key, len(data))
	if e != nil {
		problem(w, 500, "入库失败")
		return
	}
	id, _ := result.LastInsertId()
	if e = s.inject("knowledge-insert"); e != nil {
		problem(w, 500, "入库失败")
		return
	}
	result, e = tx.ExecContext(r.Context(), "INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES(?,?,?)", id, u.ClassID, text)
	if e != nil {
		problem(w, 500, "入库失败")
		return
	}
	kid, _ := result.LastInsertId()
	if e = s.inject("commit"); e != nil {
		problem(w, 500, "入库失败")
		return
	}
	if e = tx.Commit(); e != nil {
		// Ambiguous commit outcomes must not delete a file already referenced by DB.
		var stored int64
		queryErr := s.DB.QueryRow("SELECT m.id FROM materials m JOIN knowledge_entries k ON k.material_id=m.id WHERE m.storage_key=?", key).Scan(&stored)
		if queryErr == nil {
			keep = true
			writeJSON(w, 201, map[string]int64{"material_id": id, "knowledge_entry_id": kid})
			return
		} else if !errors.Is(queryErr, sql.ErrNoRows) {
			keep = true
			s.unhealthy.Store(true)
		}
		problem(w, 500, "入库失败")
		return
	}
	keep = true
	writeJSON(w, 201, map[string]int64{"material_id": id, "knowledge_entry_id": kid})
}
