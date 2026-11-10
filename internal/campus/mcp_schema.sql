CREATE TABLE IF NOT EXISTS mcp_servers (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 slug VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 display_name VARCHAR(100) NOT NULL, base_url VARCHAR(255) NOT NULL,
 credential VARBINARY(1024) NULL, status VARCHAR(16) NOT NULL DEFAULT 'disconnected',
 revision BIGINT NOT NULL DEFAULT 0, last_error_code VARCHAR(64) NULL,
 UNIQUE KEY mcp_server_class(id,class_id), UNIQUE KEY mcp_server_slug(class_id,slug),
 FOREIGN KEY(class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS mcp_discovered_tools (
 server_id BIGINT NOT NULL, class_id BIGINT NOT NULL, revision BIGINT NOT NULL,
 tool_name VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 description VARCHAR(400) NOT NULL, input_schema JSON NOT NULL, fingerprint CHAR(64) NOT NULL,
 PRIMARY KEY(server_id,tool_name),
 FOREIGN KEY(server_id,class_id) REFERENCES mcp_servers(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS assistant_tool_grants (
 assistant_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 ref VARCHAR(97) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, server_id BIGINT NULL,
 tool_name VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, fingerprint CHAR(64) NOT NULL,
 PRIMARY KEY(assistant_id,ref),
 FOREIGN KEY(assistant_id,class_id) REFERENCES assistants(id,class_id),
 FOREIGN KEY(server_id,class_id) REFERENCES mcp_servers(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS skill_tool_refs (
 skill_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 ref VARCHAR(97) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, server_id BIGINT NULL,
 tool_name VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(skill_id,ref),
 FOREIGN KEY(skill_id,class_id) REFERENCES skills(id,class_id),
 FOREIGN KEY(server_id,class_id) REFERENCES mcp_servers(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS tool_call_audit (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, attempt_id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 terminal BOOLEAN NOT NULL, class_id BIGINT NOT NULL, user_id BIGINT NOT NULL,
 conversation_id BIGINT NOT NULL, turn_id BIGINT NOT NULL, server_id BIGINT NULL,
 tool_ref VARCHAR(97) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, stage INT NOT NULL,
 status VARCHAR(16) NOT NULL, reason_code VARCHAR(64) NOT NULL DEFAULT '',
 summary VARCHAR(255) NOT NULL DEFAULT '', parameter_summary VARCHAR(80) NOT NULL DEFAULT '',
 duration_ms BIGINT NOT NULL DEFAULT 0,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY tool_attempt_state(attempt_id,terminal),
 KEY tool_audit_class(class_id,id), KEY tool_audit_turn(turn_id,id),
 KEY tool_audit_quota(conversation_id,terminal,status),
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(user_id) REFERENCES users(id),
 FOREIGN KEY(conversation_id) REFERENCES ai_conversations(id), FOREIGN KEY(turn_id) REFERENCES ai_turns(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
