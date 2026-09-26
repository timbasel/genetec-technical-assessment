package books

import (
	"fmt"
	"slices"
)

func creationChanges(book Book) []Change {
	fields := []struct {
		name        string
		value       any
		description string
	}{
		{"title", book.Title, fmt.Sprintf("Title set to %q", book.Title)},
		{"description", book.Description, fmt.Sprintf("Description set to %q", book.Description)},
		{"publication_date", book.PublicationDate, fmt.Sprintf("Publication date set to %q", book.PublicationDate)},
		{"authors", book.Authors, fmt.Sprintf("Authors set to %q", book.Authors)},
	}

	changes := make([]Change, 0, len(fields))
	for _, field := range fields {
		changes = append(changes, Change{
			BookID:      book.ID,
			OccurredAt:  book.CreatedAt,
			Kind:        "created",
			Field:       field.name,
			NewValue:    field.value,
			Description: field.description,
		})
	}
	return changes
}

func updatedChanges(before, after Book) []Change {
	changes := make([]Change, 0, 4)
	if before.Title != after.Title {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "title",
			OldValue:    before.Title,
			NewValue:    after.Title,
			Description: fmt.Sprintf("Title changed from %q to %q", before.Title, after.Title),
		})
	}
	if before.Description != after.Description {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "description",
			OldValue:    before.Description,
			NewValue:    after.Description,
			Description: fmt.Sprintf("Description changed from %q to %q", before.Description, after.Description),
		})
	}
	if before.PublicationDate != after.PublicationDate {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "publication_date",
			OldValue:    before.PublicationDate,
			NewValue:    after.PublicationDate,
			Description: fmt.Sprintf("Publication date changed from %q to %q", before.PublicationDate, after.PublicationDate),
		})
	}
	if !slices.Equal(before.Authors, after.Authors) {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "authors",
			OldValue:    before.Authors,
			NewValue:    after.Authors,
			Description: describeAuthors(before.Authors, after.Authors),
		})
	}
	return changes
}

func describeAuthors(before, after []string) string {
	if len(after) == len(before)+1 {
		for i, author := range after {
			if slices.Equal(before[:i], after[:i]) && slices.Equal(before[i:], after[i+1:]) {
				return fmt.Sprintf("Author %q was added", author)
			}
		}
	}
	if len(before) == len(after)+1 {
		for i, author := range before {
			if slices.Equal(before[:i], after[:i]) && slices.Equal(before[i+1:], after[i:]) {
				return fmt.Sprintf("Author %q was removed", author)
			}
		}
	}
	return fmt.Sprintf("Authors changed from %q to %q", before, after)
}
