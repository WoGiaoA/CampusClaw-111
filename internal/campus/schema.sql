CREATE TABLE IF NOT EXISTS classes (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, code VARCHAR(20) NOT NULL UNIQUE, name VARCHAR(100) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS users (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, username VARCHAR(100) NOT NULL UNIQUE,
 nickname VARCHAR(24) NULL,
 password_hash VARCHAR(255) NOT NULL, must_change_password BOOLEAN NOT NULL DEFAULT FALSE, role VARCHAR(20) NOT NULL,
 class_id BIGINT NOT NULL, UNIQUE KEY user_class (id,class_id),
 CONSTRAINT user_role CHECK (role IN ('teacher','student')),
 FOREIGN KEY (class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS sessions (
 id_hash CHAR(64) PRIMARY KEY, user_id BIGINT NOT NULL,
 expires_at DATETIME(6) NOT NULL, csrf_token VARCHAR(64) NOT NULL,
 FOREIGN KEY (user_id) REFERENCES users(id), INDEX (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS materials (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 uploader BIGINT NOT NULL, title VARCHAR(255) NOT NULL,
 original_filename VARCHAR(255) NOT NULL, storage_key VARCHAR(255) NOT NULL UNIQUE,
 seed_key VARCHAR(30) NULL UNIQUE, byte_size BIGINT NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY material_class (id,class_id), INDEX (class_id),
 FOREIGN KEY (class_id) REFERENCES classes(id),
 FOREIGN KEY (uploader,class_id) REFERENCES users(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS knowledge_entries (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, material_id BIGINT NOT NULL UNIQUE,
 class_id BIGINT NOT NULL, body_text MEDIUMTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), INDEX (class_id),
 FOREIGN KEY (material_id,class_id) REFERENCES materials(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS lectures (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 title VARCHAR(255) NOT NULL, description TEXT NULL,
 FOREIGN KEY(class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS assignments (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 title VARCHAR(255) NOT NULL, description TEXT NULL, due_at DATETIME NULL,
 FOREIGN KEY(class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS assistants (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 name VARCHAR(255) NOT NULL, prompt_text TEXT NULL, UNIQUE KEY assistant_class(id,class_id),
 FOREIGN KEY(class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS skills (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 assistant_id BIGINT NULL, name VARCHAR(255) NOT NULL, description TEXT NULL,
 FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(assistant_id,class_id) REFERENCES assistants(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- RETRIEVAL TABLES
CREATE TABLE IF NOT EXISTS knowledge_index_jobs (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 material_id BIGINT NOT NULL UNIQUE, knowledge_entry_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 generation BIGINT NOT NULL DEFAULT 1, status VARCHAR(10) NOT NULL DEFAULT 'pending',
 model_signature CHAR(64) NOT NULL DEFAULT '', last_error_code VARCHAR(60) NOT NULL DEFAULT '',
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
 UNIQUE KEY job_generation(id,generation,material_id,knowledge_entry_id,class_id),
 FOREIGN KEY(knowledge_entry_id,material_id,class_id) REFERENCES knowledge_entries(id,material_id,class_id),
 CHECK(status IN ('pending','running','ready','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS knowledge_chunks (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, index_job_id BIGINT NOT NULL, generation BIGINT NOT NULL,
 material_id BIGINT NOT NULL, knowledge_entry_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 chunk_index INT NOT NULL, chunk_text TEXT NOT NULL, char_start INT NOT NULL, char_end INT NOT NULL,
 index_status VARCHAR(10) NOT NULL DEFAULT 'pending',
 UNIQUE KEY chunk_ordinal(index_job_id,generation,chunk_index), INDEX(class_id,index_status),
 FOREIGN KEY(index_job_id,generation,material_id,knowledge_entry_id,class_id) REFERENCES knowledge_index_jobs(id,generation,material_id,knowledge_entry_id,class_id),
 FULLTEXT KEY chunk_fulltext(chunk_text) WITH PARSER ngram,
 CHECK(index_status IN ('pending','ready','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
