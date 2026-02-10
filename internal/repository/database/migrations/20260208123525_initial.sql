

-- +goose Up
CREATE TYPE message_status AS ENUM ('SENT', 'DELIVERED', 'READ');
CREATE TYPE group_member_status AS ENUM ('ADMIN', 'MEMBER');

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    nickname TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    from_user_id INT REFERENCES users(id) NOT NULL,
    to_user_id INT REFERENCES users(id) NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    delivered_at TIMESTAMP NULL,
    read_at TIMESTAMP NULL,
    status message_status DEFAULT 'SENT'
);

CREATE TABLE groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(30) NOT NULL,
    author_id INT REFERENCES users(id) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE group_messages (
    id SERIAL PRIMARY KEY,
    group_id INT REFERENCES groups(id) ON DELETE CASCADE NOT NULL,
    from_user_id INT REFERENCES users(id) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id INT REFERENCES groups(id) ON DELETE CASCADE NOT NULL,
    user_id INT REFERENCES users(id) NOT NULL,
    role group_member_status DEFAULT 'MEMBER',
    joined_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY(group_id, user_id)
);

CREATE TABLE group_message_status (
    message_id INT REFERENCES group_messages(id) ON DELETE CASCADE NOT NULL,
    user_id INT REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    status message_status DEFAULT 'SENT' NOT NULL,
    delivered_at TIMESTAMP NULL,
    read_at TIMESTAMP NULL,
    PRIMARY KEY (message_id, user_id)
);

INSERT INTO users (nickname) VALUES 
    ('colorvax-1'),
    ('colorvax-2'),
    ('colorvax-3'),
    ('colorvax-4'),
    ('colorvax-5');

-- +goose Down
DROP TABLE IF EXISTS group_message_status;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS group_messages;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS messages;
DROP TYPE IF EXISTS group_member_status;
DROP TYPE IF EXISTS message_status;
DROP TABLE IF EXISTS users;
