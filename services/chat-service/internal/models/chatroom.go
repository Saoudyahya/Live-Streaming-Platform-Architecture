package models

import "time"

type Chatroom struct {
	ID          string    `json:"id" dynamodbav:"id"`
	Name        string    `json:"name" dynamodbav:"name"`
	Description string    `json:"description" dynamodbav:"description"`
	CreatorID   string    `json:"creator_id" dynamodbav:"creator_id"`
	IsPrivate   bool      `json:"is_private" dynamodbav:"is_private"`
	MemberIDs   []string  `json:"member_ids" dynamodbav:"member_ids"`
	CreatedAt   time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" dynamodbav:"updated_at"`
}
