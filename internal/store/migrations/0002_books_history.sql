CREATE TABLE books_history (
  id INTEGER PRIMARY KEY,
  book_id TEXT NOT NULL REFERENCES books(id),
  occurred_at INTEGER NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('created', 'updated')),
  field TEXT NOT NULL CHECK(field IN ('title', 'description', 'publication_date', 'authors')),
  old_value TEXT CHECK(old_value IS NULL OR json_valid(old_value)),
  new_value TEXT NOT NULL CHECK(json_valid(new_value)),
  description TEXT NOT NULL
);

CREATE INDEX history_book_id ON books_history(book_id, id);
CREATE INDEX history_book_field ON books_history(book_id, field, id);
CREATE INDEX history_book_time ON books_history(book_id, occurred_at, id);

CREATE TRIGGER books_history_no_update BEFORE UPDATE ON books_history
BEGIN SELECT RAISE(ABORT, 'books_history is append-only'); END;
CREATE TRIGGER books_history_no_delete BEFORE DELETE ON books_history
BEGIN SELECT RAISE(ABORT, 'books_history is append-only'); END;
