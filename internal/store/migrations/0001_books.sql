CREATE TABLE books (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  publication_date TEXT NOT NULL,
  authors TEXT NOT NULL CHECK(json_valid(authors)),
  version INTEGER NOT NULL CHECK(version > 0),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

 CREATE INDEX books_creation ON books(created_at, id);
