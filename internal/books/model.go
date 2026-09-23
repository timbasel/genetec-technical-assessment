package books

import "time"

type Book struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	PublicationDate string    `json:"publicationDate"`
	Authors         []string  `json:"authors"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Change struct {
	ID          int64     `json:"id"`
	BookID      string    `json:"bookID"`
	OccurredAt  time.Time `json:"occuredAt"`
	Kind        string    `json:"kind"`
	Field       string    `json:"field"`
	OldValue    any       `json:"oldValue"`
	NewValue    any       `json:"newValue"`
	Description string    `json:"description"`
}

type Query struct {
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
	Kind   string     `json:"kind"`
	Field  string     `json:"field"`
	From   *time.Time `json:"from"`
	To     *time.Time `json:"to"`
	Order  string     `json:"order"`
}

type Page struct {
}
