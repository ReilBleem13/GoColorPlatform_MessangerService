-- +goose Up
CREATE TYPE message_status AS ENUM ('SENT', 'DELIVERED', 'READ');
CREATE TYPE chat_type AS ENUM ('PRIVATE', 'GROUP');
CREATE TYPE message_type AS ENUM ('MESSAGE', 'NEW_MEMBER', 'LEFT_MEMBER', 'KICKED_MEMBER')

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    nickname TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE chats (
    id SERIAL PRIMARY KEY,
    type chat_type NOT NULL,
    name VARCHAR(100),                  -- NULL для приватных чатов, название для групп
    author_id INT REFERENCES users(id), -- NULL для приватных, создатель для групп
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE chat_members (
    chat_id INT REFERENCES chats(id) ON DELETE CASCADE NOT NULL,
    user_id INT REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    role VARCHAR(20) DEFAULT 'MEMBER', 
    joined_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY(chat_id, user_id)
);

CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    chat_id INT REFERENCES chats(id) ON DELETE CASCADE NOT NULL,
    from_user_id INT REFERENCES users(id) NOT NULL,
    message_type TEXT DEFAULT 'MESSAGE'
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE message_status (
    message_id INT REFERENCES messages(id) ON DELETE CASCADE NOT NULL,
    user_id INT REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    status message_status DEFAULT 'SENT' NOT NULL,
    delivered_at TIMESTAMP NULL,
    read_at TIMESTAMP NULL,
    PRIMARY KEY (message_id, user_id)
);

CREATE OR REPLACE FUNCTION update_chat_timestamp()
RETURNS TRIGGER AS &&
BEGIN
    UPDATE chats SET updated_at = NEW.created_at WHERE id = NEW.chat_id;
    RETURN NEW;
END;
&& LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_chat_timestamp
AFTER INSERT ON messages
FOR EACH ROW
EXECUTE FUNCTION update_chat_timestamp();

INSERT INTO users (nickname) VALUES 
    ('colorvax-1'),
    ('colorvax-2'),
    ('colorvax-3'),
    ('colorvax-4'),
    ('colorvax-5');

-- +goose Down
DROP TRIGGER IF EXISTS trigger_update_chat_timestamp ON messages;
DROP FUNCTION IF EXISTS update_chat_timestamp();
DROP INDEX IF EXISTS idx_chats_updated_at;
DROP INDEX IF EXISTS idx_message_status_user_message;
DROP INDEX IF EXISTS idx_chat_members_chat_id;
DROP INDEX IF EXISTS idx_chat_members_user_id;
DROP INDEX IF EXISTS idx_messages_chat_id_created_at;
DROP TABLE IF EXISTS message_status;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_members;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS chat_type;
DROP TYPE IF EXISTS message_status;

