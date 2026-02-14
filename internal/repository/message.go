package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/ReilBleem13/MessangerV2/internal/service"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type MessageRepo struct {
	db    *sqlx.DB
	cache *redis.Client
}

func NewMessageRepo(db *sqlx.DB, cache *redis.Client) *MessageRepo {
	return &MessageRepo{
		db:    db,
		cache: cache,
	}
}

// To add messages from private chats and group chats.
func (mp *MessageRepo) NewMessage(ctx context.Context, in *domain.Message) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO messages (
			chat_id,
			from_user_id,
			event_type,
			content
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id;
	`

	var messageID int
	err = tx.QueryRowContext(ctx, query,
		in.ChatID,
		in.FromUserID,
		string(in.MessageType),
		in.Content,
	).Scan(&messageID)
	if err != nil {
		return 0, err
	}

	members, err := mp.getAllChatMembersWithExecutor(ctx, tx, in.ChatID)
	if err != nil {
		return 0, err
	}

	statusQuery := `
		INSERT INTO message_status (
			message_id, 
			user_id, 
			status
		)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id) DO NOTHING;
	`

	for _, member := range members {
		_, err := tx.ExecContext(ctx, statusQuery,
			messageID,
			member.ID,
			string(domain.StatusSent),
		)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return messageID, nil
}

func (mp *MessageRepo) EditMessage(ctx context.Context, messageID int, content string) error {
	query := `
		UPDATE messages
		SET content = $1, updated_at = NOW()
		WHERE id = $2;
	`

	_, err := mp.db.ExecContext(ctx, query,
		content,
		messageID,
	)
	return err
}

func (mp *MessageRepo) DeleteMessage(ctx context.Context, messageID int) error {
	query := `
		DETELE FROM messages
		WHERE id = $1
	`

	_, err := mp.db.ExecContext(ctx, query,
		messageID,
	)
	return err
}

func (mp *MessageRepo) GetMessageAuthorID(ctx context.Context, messageID int) (int, error) {
	query := `
		SELECT from_user_id
		FROM messages
		WHERE id = $1
	`

	var userID int
	if err := mp.db.GetContext(ctx, &userID, query,
		messageID,
	); err != nil {
		return 0, err
	}
	return userID, nil
}

func (mp *MessageRepo) NewGroupChat(ctx context.Context, name string, authorID int) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO chats (type, name, author_id) 
		VALUES ($1, $2, $3)
		RETURNING id;
	`

	var groupID int
	err = tx.QueryRowContext(ctx, query,
		string(domain.Group),
		name,
		authorID,
	).Scan(&groupID)
	if err != nil {
		return 0, err
	}

	query = `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3);
	`

	_, err = tx.ExecContext(ctx, query,
		groupID,
		authorID,
		string(domain.AdminRole),
	)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return groupID, err
}

func (mp *MessageRepo) DeleteGroupChat(ctx context.Context, chatID, authorID int) error {
	query := `
		DELETE FROM chats 
		WHERE id = $1 AND author_id = $2;
	`

	res, err := mp.db.ExecContext(ctx, query,
		chatID,
		authorID,
	)
	if err != nil {
		return err
	}

	rowsAff, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAff == 0 {
		return fmt.Errorf("user %d is not author of %d group chat", authorID, chatID)
	}
	return nil
}

func (mp *MessageRepo) NewGroupChatMember(ctx context.Context, chatID, userID int) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO chat_members (chat_id, user_id)
		VALUES ($1, $2);
	`

	_, err = tx.ExecContext(ctx, query,
		chatID,
		userID,
	)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, domain.ErrAlreadyExists
		}
		return 0, err
	}

	query = `
		INSERT INTO messages (chat_id, from_user_id, event_type)
		VALUES ($1, $2, $3)
		RETURNING id;
	`

	var messageID int
	err = tx.QueryRowContext(ctx, query,
		chatID,
		userID,
		string(domain.NewMemberType),
	).Scan(&messageID)
	if err != nil {
		return 0, err
	}

	members, err := mp.GetAllChatMembers(ctx, chatID)
	if err != nil {
		return 0, err
	}

	statusQuery := `
		INSERT INTO message_status (message_id, user_id, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id) DO NOTHING;
	`

	for _, member := range members {
		_, err := tx.ExecContext(ctx, statusQuery,
			messageID,
			member.ID,
			string(domain.StatusSent),
		)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return messageID, nil
}

// typeDelete => LeftMemberType или KickedMemberType
func (mp *MessageRepo) DeleteGroupMember(ctx context.Context, chatID, userID int, typeDelete domain.EventType) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		DELETE FROM chat_members 
		WHERE chat_id = $1 AND user_id = $2;
	`

	_, err = tx.ExecContext(ctx, query,
		chatID,
		userID,
	)
	if err != nil {
		return 0, err
	}

	query = `
		INSERT INTO messages (chat_id, from_user_id, event_type)
		VALUES($1, $2, $3)
		RETURNING id;
	`

	var messageID int
	err = tx.QueryRowContext(ctx, query,
		chatID,
		userID,
		string(typeDelete),
	).Scan(&messageID)
	if err != nil {
		return 0, err
	}

	members, err := mp.GetAllChatMembers(ctx, chatID)
	if err != nil {
		return 0, err
	}

	statusQuery := `
		INSERT INTO message_status (message_id, user_id, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id) DO NOTHING;
	`

	for _, member := range members {
		_, err := tx.ExecContext(ctx, statusQuery,
			messageID,
			member.ID,
			string(domain.StatusSent),
		)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return messageID, nil
}

func (mp *MessageRepo) GetAllChatMembers(ctx context.Context, chatID int) ([]*domain.ChatMember, error) {
	return mp.getAllChatMembersWithExecutor(ctx, mp.db, chatID)
}

func (mp *MessageRepo) getAllChatMembersWithExecutor(ctx context.Context, executor sqlx.ExtContext, chatID int) ([]*domain.ChatMember, error) {
	query := `
		SELECT 
			cm.user_id AS id,
			u.nickname
		FROM chat_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.chat_id = $1
	`

	var chatMembers []*domain.ChatMember
	err := sqlx.SelectContext(ctx, executor, &chatMembers, query,
		chatID,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return chatMembers, nil
}

func (mp *MessageRepo) GetOrCreatePrivateChat(ctx context.Context, userID1, userID2 int) (int, bool, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	query := `
		SELECT c.id
		FROM chats c
		JOIN chat_members cm1 ON cm1.chat_id = c.id
		JOIN chat_members cm2 ON cm2.chat_id = c.id
		WHERE c.type = $1
			AND cm1.user_id = $2
			AND cm2.user_id = $3
			AND cm1.user_id != cm2.user_id
	`

	var chatID int
	err = tx.GetContext(ctx, &chatID, query,
		string(domain.Private),
		userID1,
		userID2,
	)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return 0, false, err
		}
		return chatID, false, nil
	}

	if err != sql.ErrNoRows {
		return 0, false, err
	}

	query = `
		INSERT INTO chats (type)
		VALUES ($1)
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, query,
		string(domain.Private),
	).Scan(&chatID)
	if err != nil {
		return 0, false, err
	}

	query = `
		INSERT INTO chat_members (chat_id, user_id)
		VALUES ($1, $2)
	`

	for _, userID := range []int{userID1, userID2} {
		_, err = tx.ExecContext(ctx, query,
			chatID,
			userID,
		)
		if err != nil {
			return 0, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return chatID, true, nil
}

func (mp *MessageRepo) GetUserChats(ctx context.Context, userID int) ([]domain.UserChat, error) {
	query := `
		SELECT 
			c.id,
			c.type, 
			c.name, 
			c.author_id, 
			c.created_at
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id
		WHERE cm.user_id = $1
		ORDER BY c.updated_at DESC
	`

	var userChats []domain.UserChat
	err := mp.db.SelectContext(ctx, &userChats, query,
		userID,
	)
	if err != nil {
		return nil, err
	}
	return userChats, nil
}

func (mp *MessageRepo) GetGroupChatMemberRole(ctx context.Context, userID, chatID int) (domain.GroupMemberRole, error) {
	query := `
		SELECT 
			role
		FROM chat_members
		WHERE chat_id = $1
			AND user_id = $2
	`

	var role domain.GroupMemberRole
	if err := mp.db.GetContext(ctx, &role, query,
		chatID,
		userID,
	); err != nil {
		return "", err
	}

	return role, nil
}

func (mp *MessageRepo) ChangeGroupChatMemberRole(ctx context.Context, in *service.UpdateGroupMemberRoleDTO) error {
	query := `
		UPDATE chat_members 
			SET role = $1
		WHERE chat_id = $2 AND user_id = $3
	`

	_, err := mp.db.ExecContext(ctx, query,
		string(in.Role),
		in.ChatID,
		in.ObjectID,
	)
	return err
}

func (mp *MessageRepo) GetAllUndeliveredMessages(ctx context.Context, userID int) ([]domain.Message, error) {
	query := `
		SELECT 
			m.id,
			m.chat_id,
			m.from_user_id,
			m.event_type as message_type,
			m.content,
			m.created_at
		FROM messages m 
		JOIN message_status ms ON ms.message_id = m.id 
		WHERE ms.user_id = $1 AND ms.status = $2
		ORDER BY id DESC
	`
	var messages []domain.Message
	err := mp.db.SelectContext(ctx, &messages, query,
		userID,
		string(domain.StatusSent),
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return messages, nil
}

func (mp *MessageRepo) PaginateMessages(ctx context.Context, chatID int, cursor *int) ([]domain.Message, *int, bool, error) {
	query := `
		SELECT 
			id,
			chat_id,
			from_user_id,
			event_type as message_type,
			content,
			created_at
		FROM messages 
		WHERE chat_id = $1 
			AND ($2::INTEGER IS NULL OR id < $2)
		ORDER BY id DESC
		LIMIT 21
	`

	var messages []domain.Message
	err := mp.db.SelectContext(ctx, &messages, query,
		chatID,
		cursor,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, false, err
	}

	hasMore := len(messages) > 20
	if hasMore {
		messages = messages[:20]
	}

	var nextCursor *int
	if len(messages) > 0 {
		lastID := messages[len(messages)-1].ID
		nextCursor = &lastID
	}
	return messages, nextCursor, hasMore, nil
}

func (mp *MessageRepo) SetDeliveredAtStatus(ctx context.Context, messageID, userID int) error {
	query := `
		UPDATE message_status
			SET status = 'DELIVERED', delivered_at = NOW()
		WHERE message_id = $2 
			AND user_id = $3;
	`

	_, err := mp.db.ExecContext(ctx, query,
		messageID,
		userID,
	)
	if err != nil {
		return err
	}
	return nil
}

func (mp *MessageRepo) SetReadAtStatus(ctx context.Context, upToID, chatID, userID int) error {
	query := `
		UPDATE message_status ms
		SET status = 'READ', read_at = NOW()
		FROM messages m
		WHERE ms.message_id = m.id
			AND m.chat_id = $1
			AND ms.message_id <= $2
			AND ms.user_id = $3
			AND ms.status = 'DELIVERED'
	`

	_, err := mp.db.ExecContext(ctx, query,
		chatID,
		upToID,
		userID,
	)
	return err
}

func (mp *MessageRepo) GetUserContacts(ctx context.Context, userID int) ([]int, error) {
	query := `
		SELECT 
			cm2.user_id
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id
		JOIN chat_members cm2 ON cm2.chat_id = c.id
		WHERE c.type = $1
			AND cm.user_id = $2
			AND cm2.user_id != $2
	`

	var contacts []int
	err := mp.db.SelectContext(ctx, &contacts, query,
		domain.Private,
		userID,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return contacts, nil
}
