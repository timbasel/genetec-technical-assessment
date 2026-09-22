# Assessment Task

Build a small Go backend service that manages books and keeps a history of the changes made to them.

## Book Model

Each book should contain:

- An ID
- A title
- A short description
- A publication date
- One or more authors

## Core functionality

- Create, retrieve, and update books through an HTTP API.
- Record a history entry whenever a book is changed.
- Expose the change history through a dedicated endpoint.

We are particularly interested in how you model the change history. There is no required approach choose one that makes sense and be prepared to discuss its trade-offs.

## Change history

Every history entry should include at least the time of the change and a clear, human-readable description of what changed.

```
Title changed from "The Hobbitt" to "The Hobbit"
Author "J.R.R. Tolkien" was added
```

The history endpoint should support pagination, filtering, and ordering. Grouping or other query capabilities are optional if you believe they add useful value.

## Technical expectations

- **API documentation:** Provide an OpenAPI/Swagger specification for the HTTP API.
- **Interactive API UI:** Provide a browser-based interface that uses the generated OpenAPI specification to dynamically create a UI for exploring and testing the API endpoints. You may use any suitable technology.
- **Go design:** Organize the code as you would for a production service. You may use the standard library or a framework of your choice.
- **Context:** Propagate context.Context through handlers, services, and storage operations where applicable.
- **HTTP behavior:** Handle errors and status codes appropriately.
- **Persistence:** Use any storage mechanism you consider appropriate.

## Tests and documentation

Comprehensive test coverage and extensive documentation are not required. A few focused tests around important behavior are welcome.

Include a short README explaining how to run the service and describing any significant design decisions or trade-offs.

## Submission

This task is not confidential. You may host the project in a public Git repository or share the code in another convenient way. Please include the Git commit history.

## Last thoughts

This task is designed to showcase your technical abilities, creativity, and problem-solving skills - but most importantly, it is an opportunity to enjoy building something. Feel free to make thoughtful choices and bring your own approach to the solution.

We look forward to seeing your approach!
