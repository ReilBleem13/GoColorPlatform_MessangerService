-- +goose Up


CREATE TYPE message_delivery_status AS ENUM ('SENT', 'DELIVERED', 'READ');
CREATE TYPE chat_type AS ENUM ('PRIVATE', 'GROUP');
CREATE TYPE chat_event_type AS ENUM ('left_member', 'kicked_member', 'new_member', 'new_message');
--------------------------------------------------------------------------------

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    nickname VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    hash_password TEXT NOT NULL,
    about TEXT,
    experience TEXT,
    github_url VARCHAR(500),
    linkedin_url VARCHAR(500),
    twitter_url VARCHAR(500),
    telegram_url VARCHAR(500),
    website_url VARCHAR(500),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chats (
    id          SERIAL PRIMARY KEY,
    type        chat_type NOT NULL,
    name        VARCHAR(100),                    
    author_id   INT REFERENCES users(id),         
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE chat_members (
    chat_id     INT REFERENCES chats(id) ON DELETE CASCADE NOT NULL,
    user_id     INT REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    role        VARCHAR(20) DEFAULT 'MEMBER' NOT NULL,
    joined_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE messages (
    id              SERIAL PRIMARY KEY,
    chat_id         INT REFERENCES chats(id) ON DELETE CASCADE NOT NULL,
    from_user_id    INT REFERENCES users(id) ON DELETE SET NULL,  
    content         TEXT,                                 
    event_type      chat_event_type,                                
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE message_status (
    message_id      INT REFERENCES messages(id) ON DELETE CASCADE NOT NULL,
    user_id         INT REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    status          message_delivery_status DEFAULT 'SENT' NOT NULL,
    delivered_at    TIMESTAMP WITH TIME ZONE,
    read_at         TIMESTAMP WITH TIME ZONE,

    PRIMARY KEY (message_id, user_id)
);

CREATE TABLE refresh_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_id VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP NULL
);

--------------------------------------------------------------------------------
-- Триггер обновления updated_at в чате при новом сообщении/событии

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_chat_timestamp()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE chats 
       SET updated_at = NEW.created_at 
     WHERE id = NEW.chat_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER trigger_update_chat_timestamp
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION update_chat_timestamp();

--------------------------------------------------------------------------------
-- Индексы

CREATE INDEX idx_messages_chat_id_created_at   ON messages (chat_id, created_at DESC);
CREATE INDEX idx_messages_from_user_id         ON messages (from_user_id);
CREATE INDEX idx_chat_members_user_id          ON chat_members (user_id);
CREATE INDEX idx_chat_members_chat_id_role     ON chat_members (chat_id, role);
CREATE INDEX idx_message_status_user_id_status ON message_status (user_id, status);
CREATE INDEX idx_chats_type_updated_at         ON chats (type, updated_at DESC);

--------------------------------------------------------------------------------

-- +goose Down

DROP TRIGGER IF EXISTS trigger_update_chat_timestamp ON messages;
DROP FUNCTION  IF EXISTS update_chat_timestamp();

DROP INDEX IF EXISTS idx_chats_type_updated_at;
DROP INDEX IF EXISTS idx_message_status_user_id_status;
DROP INDEX IF EXISTS idx_chat_members_chat_id_role;
DROP INDEX IF EXISTS idx_chat_members_user_id;
DROP INDEX IF EXISTS idx_messages_from_user_id;
DROP INDEX IF EXISTS idx_messages_chat_id_created_at;

DROP TABLE IF EXISTS message_status;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_members;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS chat_event_type;
DROP TYPE IF EXISTS chat_type;
DROP TYPE IF EXISTS message_delivery_status;