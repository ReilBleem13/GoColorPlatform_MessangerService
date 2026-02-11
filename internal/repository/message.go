package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/ReilBleem13/MessangerV2/internal/service"
	"github.com/jmoiron/sqlx"
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

func (mp *MessageRepo) NewMessage(ctx context.Context, fromUserID int, in *service.PrivateMessage) (int, error) {
	query := `
		INSERT INTO messages (
			from_user_id,
			to_user_id,
			content,
			created_at
		)
		VALUES ($1, $2, $3, NOW())
		RETURNING id;
	`
	fmt.Printf("тута")
	var messageID int
	err := mp.db.QueryRowContext(ctx, query, fromUserID, in.ToUserID, in.Content).Scan(&messageID)
	fmt.Printf("снова тута")
	return messageID, err
}

func (mp *MessageRepo) UpdateMessageStatus(ctx context.Context, messageID int, status domain.MessageStatus) error {
	query := `
		UPDATE messages
		SET status = $1
		WHERE id = $2;
	`

	_, err := mp.db.ExecContext(ctx, query, string(status), messageID)
	return err
}

// groups
func (mp *MessageRepo) NewGroup(ctx context.Context, name string, authorID int) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO groups (name, author_id) 
		VALUES ($1, $2)
		RETURNING id;
	`

	var groupID int
	err = tx.QueryRowContext(ctx, query, name, authorID).Scan(&groupID)
	if err != nil {
		return 0, err
	}

	query = `
		INSERT INTO group_members (group_id, user_id, role)
		VALUES ($1, $2, 'ADMIN');
	`

	_, err = tx.ExecContext(ctx, query, groupID, authorID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return groupID, err
}

func (mp *MessageRepo) DeleteGroup(ctx context.Context, userID, groupID int) error {
	query := `
		DELETE FROM groups WHERE id = $1 AND author_id = $2;
	`

	res, err := mp.db.ExecContext(ctx, query, groupID, userID)
	if err != nil {
		return err
	}

	rowsAff, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAff == 0 {
		return fmt.Errorf("user %d is not author %d group", userID, groupID)
	}
	return nil
}

func (mp *MessageRepo) NewGroupMember(ctx context.Context, groupID, userID int) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO group_members (group_id, user_id)
		VALUES ($1, $2);
	`

	_, err = tx.ExecContext(ctx, query, groupID, userID)
	if err != nil {
		return 0, err
	}

	query = `
		INSERT INTO group_messages (group_id, from_user_id, message_type)
		VALUES ($1, $2, 'NEW_MEMBER')
		RETURNING id;
	`

	var messageID int
	err = tx.QueryRowContext(ctx, query, groupID, userID).Scan(&messageID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return messageID, nil
}

func (mp *MessageRepo) DeleteGroupMember(ctx context.Context, groupID int, userID int) (int, error) {
	tx, err := mp.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;
	`

	_, err = tx.ExecContext(ctx, query, groupID, userID)
	if err != nil {
		return 0, err
	}

	query = `
		INSERT INTO group_messages (group_id, from_user_id, message_type)
		VALUES($1, $2, 'EXIT_MEMBER')
		RETURNING id;
	`

	var messageID int
	err = tx.QueryRowContext(ctx, query, groupID, userID).Scan(&messageID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return messageID, nil
}

func (mp *MessageRepo) GetAllGroupMembers(ctx context.Context, groupID int) ([]int, error) {
	query := `
		SELECT user_id FROM group_members WHERE group_id = $1
	`

	var groupMembersIDs []int
	err := mp.db.SelectContext(ctx, &groupMembersIDs, query, groupID)
	if err != nil {
		return nil, err
	}
	return groupMembersIDs, nil
}

func (mp *MessageRepo) GetUserGroups(ctx context.Context, userID int) ([]service.UserGroup, error) {
	query := `
		SELECT g.id, g.name 
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.created_at DESC
	`

	var userGroups []service.UserGroup
	err := mp.db.SelectContext(ctx, &userGroups, query, userID)
	if err != nil {
		return nil, err
	}
	return userGroups, nil
}

func (mp *MessageRepo) ChangeGroupMemberRole(ctx context.Context, in *service.UpdateGroupMemberRoleDTO) error {
	query := `
		UPDATE group_members SET role = $1
		WHERE group_id = $2 AND user_id = $3
	`

	_, err := mp.db.ExecContext(ctx, query, string(in.Role), in.GroupID, in.UserID)
	return err
}

func (mp *MessageRepo) NewGroupMessage(ctx context.Context, groupID int, fromUserID int, content string) (int, error) {
	query := `
		INSERT INTO group_messages (group_id, from_user_id, content)
		VALUES ($1, $2, $3)
		RETURNING id;
	`

	var groupMessageID int
	err := mp.db.QueryRowContext(ctx, query, groupID, fromUserID, content).Scan(&groupMessageID)
	if err != nil {
		return 0, err
	}

	members, err := mp.GetAllGroupMembers(ctx, groupID)
	if err != nil {
		return groupMessageID, err
	}

	statusQuery := `
		INSERT INTO group_message_status (message_id, user_id, status)
		VALUES ($1, $2, 'SENT')
		ON CONFLICT (message_id, user_id) DO NOTHING;
	`

	for _, memberID := range members {
		_, err := mp.db.ExecContext(ctx, statusQuery, groupMessageID, memberID)
		if err != nil {
			return 0, err
		}
	}

	return groupMessageID, nil
}

func (mp *MessageRepo) GetAllUndeliveredMessages(ctx context.Context, userID int) ([]service.ProduceMessage, error) {
	query := `
		SELECT 
			m.id AS message_id,
			'PRIVATE_MESSAGE'::TEXT AS messsage_type,
			m.from_user_id,
			NULL::INT AS group_id,
			m.content,
			m.created_at
		FROM messages m
		WHERE m.to_user_id = $1 
			AND m.status = 'SENT'

		UNION ALL

		SELECT 
			gm.id as message_id,
			'GROUP_MESSAGE'::TEXT AS message_type,
			gm.from_user_id,
			gm.group_id,
			gm.content,
			gm.created_at
		FROM group_messages gm 
		JOIN group_message_status gms ON gms.message_id = gm_id
		WHERE gms.user_id = $1 AND gms.status = 'SENT'

		ORDER BY created_at ASC;
	`

	var messages []service.ProduceMessage
	err := mp.db.SelectContext(ctx, &messages, query, userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return messages, nil
}

func (mp *MessageRepo) PaginatePrivateMessages(ctx context.Context, userID1, userID2 int, cursor *int) ([]service.ProduceMessage, *int, bool, error) {
	var query string
	var err error
	var messages []struct {
		ID         int       `db:"id"`
		FromUserID int       `db:"from_user_id"`
		ToUserID   int       `db:"to_user_id"`
		Content    string    `db:"content"`
		CreatedAt  time.Time `db:"created_at"`
	}

	if cursor == nil {
		query = `
			SELECT 
				id,
				from_user_id,
				to_user_id,
				content,
				created_at
			FROM messages
			WHERE (
				(from_user_id = $1 AND to_user_id = $2)
				OR
				(from_user_id = $2 AND to_user_id = $1)
			)
			ORDER BY id DESC
			LIMIT 21;
		`
		err = mp.db.SelectContext(ctx, &messages, query, userID1, userID2)
	} else {
		query = `
			SELECT 
				id,
				from_user_id,
				to_user_id,
				content,
				created_at
			FROM messages
			WHERE (
				(from_user_id = $1 AND to_user_id = $2)
				OR
				(from_user_id = $2 AND to_user_id = $1)
			)
			AND id < $3
			ORDER BY id DESC
			LIMIT 21;
		`
		err = mp.db.SelectContext(ctx, &messages, query, userID1, userID2, *cursor)
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, false, err
	}

	hasMore := len(messages) > 20
	if hasMore {
		messages = messages[:20]
	}

	result := make([]service.ProduceMessage, len(messages))
	for i, msg := range messages {
		result[i] = service.ProduceMessage{
			MessageID:   msg.ID,
			TypeMessage: service.PrivateMessageType,
			FromUserID:  msg.FromUserID,
			CreatedAt:   msg.CreatedAt,
			Content:     msg.Content,
		}
	}

	var nextCursor *int
	if len(messages) > 0 {
		lastID := messages[len(messages)-1].ID
		nextCursor = &lastID
	}
	return result, nextCursor, hasMore, nil
}

func (mp *MessageRepo) PaginateGroupMessages(ctx context.Context, groupID int, cursor *int) ([]service.ProduceMessage, *int, bool, error) {
	var query string
	var err error
	var messages []struct {
		ID         int       `db:"id"`
		GroupID    int       `db:"group_id"`
		FromUserID int       `db:"from_user_id"`
		Content    string    `db:"content"`
		CreatedAt  time.Time `db:"created_at"`
	}

	if cursor == nil {
		query = `
			SELECT 
				id,
				group_id,
				from_user_id,
				content,
				created_at
			FROM group_messages
			WHERE group_id = $1
			ORDER BY id DESC
			LIMIT 21;
		`
		err = mp.db.SelectContext(ctx, &messages, query, groupID)
	} else {
		query = `
			SELECT 
				id,
				group_id,
				from_user_id,
				content,
				created_at
			FROM group_messages
			WHERE group_id = $1 AND id < $2
			ORDER BY id DESC
			LIMIT 21;
		`
		err = mp.db.SelectContext(ctx, &messages, query, groupID, *cursor)
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, false, err
	}

	hasMore := len(messages) > 20
	if hasMore {
		messages = messages[:20]
	}

	result := make([]service.ProduceMessage, len(messages))
	for i, msg := range messages {
		result[i] = service.ProduceMessage{
			MessageID:   msg.ID,
			TypeMessage: service.GroupMessageType,
			GroupID:     &msg.GroupID,
			FromUserID:  msg.FromUserID,
			CreatedAt:   msg.CreatedAt,
			Content:     msg.Content,
		}
	}

	var nextCursor *int
	if len(messages) > 0 {
		lastID := messages[len(messages)-1].ID
		nextCursor = &lastID
	}
	return result, nextCursor, hasMore, nil
}

func (mp *MessageRepo) UpdateGroupMessageStatus(ctx context.Context, messageID, userID int, status domain.MessageStatus) error {
	query := `
		UPDATE group_message_status SET status = $1
		WHERE message_id = $2 AND user_id = $3;
	`

	res, err := mp.db.ExecContext(ctx, query, string(status), messageID, userID)
	if err != nil {
		return err
	}

	rowsAff, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAff == 0 {
		return fmt.Errorf("group or user not found")
	}
	return nil
}

func (mp *MessageRepo) GetUserContacts(ctx context.Context, userID int) ([]int, error) {
	query := `
		SELECT DISTINCT to_user_id AS contact_id
		FROM messages
		WHERE from_user_id = $1
		
		UNION
		
		SELECT DISTINCT from_user_id AS contact_id
		FROM messages
		WHERE to_user_id = $1
		
		ORDER BY contact_id;
	`

	var contacts []int
	err := mp.db.SelectContext(ctx, &contacts, query, userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return contacts, nil
}
