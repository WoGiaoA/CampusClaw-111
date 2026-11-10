CREATE TABLE IF NOT EXISTS prompt_templates (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL,
 name VARCHAR(100) NOT NULL, body TEXT NOT NULL, builtin_key VARCHAR(40) NULL,
 UNIQUE KEY template_class(id,class_id), UNIQUE KEY template_builtin(class_id,builtin_key),
 FOREIGN KEY(class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- AI COLUMN MIGRATIONS
CREATE TABLE IF NOT EXISTS assistant_skills (
 assistant_id BIGINT NOT NULL, skill_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 PRIMARY KEY(assistant_id,skill_id),
 FOREIGN KEY(assistant_id,class_id) REFERENCES assistants(id,class_id),
 FOREIGN KEY(skill_id,class_id) REFERENCES skills(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS assistant_materials (
 assistant_id BIGINT NOT NULL, material_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 PRIMARY KEY(assistant_id,material_id),
 FOREIGN KEY(assistant_id,class_id) REFERENCES assistants(id,class_id),
 FOREIGN KEY(material_id,class_id) REFERENCES materials(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_migrations (
 name VARCHAR(60) PRIMARY KEY
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_conversations (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, user_id BIGINT NOT NULL, class_id BIGINT NOT NULL,
 assistant_id BIGINT NOT NULL, title VARCHAR(100) NOT NULL,
 deleted_at DATETIME(6) NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 INDEX conversation_owner(user_id,class_id,updated_at,id),
 FOREIGN KEY(user_id,class_id) REFERENCES users(id,class_id),
 FOREIGN KEY(assistant_id,class_id) REFERENCES assistants(id,class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_turns (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, conversation_id BIGINT NOT NULL,
 client_request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 status VARCHAR(10) NOT NULL, failure_code VARCHAR(60) NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY turn_request(conversation_id,client_request_id),
 INDEX conversation_turn(conversation_id,id),
 INDEX conversation_status(conversation_id,status),
 CHECK(status IN ('running','completed','failed')),
 FOREIGN KEY(conversation_id) REFERENCES ai_conversations(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_messages (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, turn_id BIGINT NOT NULL,
 role VARCHAR(10) NOT NULL, body TEXT NOT NULL, metadata JSON NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY turn_role(turn_id,role), CHECK(role IN ('user','assistant')),
 FOREIGN KEY(turn_id) REFERENCES ai_turns(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
