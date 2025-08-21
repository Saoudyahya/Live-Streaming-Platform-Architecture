package models

import "time"

type MessageType int

const (
	MessageTypeText MessageType = iota
	MessageTypeImage
	MessageTypeFile
	MessageTypeSystem
)

type Message struct {
	ID         string      `json:"id" dynamodbav:"id"`
	ChatroomID string      `json:"chatroom_id" dynamodbav:"chatroom_id"`
	UserID     string      `json:"user_id" dynamodbav:"user_id"`
	Username   string      `json:"username" dynamodbav:"username"`
	Content    string      `json:"content" dynamodbav:"content"`
	Type       MessageType `json:"type" dynamodbav:"type"`
	CreatedAt  time.Time   `json:"created_at" dynamodbav:"created_at"`
	IsEdited   bool        `json:"is_edited" dynamodbav:"is_edited"`
}
