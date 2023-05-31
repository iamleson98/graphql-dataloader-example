package main

import (
	"fmt"
	"log"

	"github.com/Masterminds/squirrel"
)

// JSONString implements JSONString custom graphql scalar type
type JSONString map[string]interface{}

func (JSONString) ImplementsGraphQLType(name string) bool {
	return name == "JSONString"
}

func (j *JSONString) UnmarshalGraphQL(input interface{}) error {
	switch t := input.(type) {
	case map[string]interface{}:
		*j = t

	default:
		return fmt.Errorf("wrong type: %T", t)
	}

	return nil
}

const txt = `
schema {
	query: Query
}

scalar JSONString

type Query {}

extend type Query {
	todos(ids: [Int!]!): [Todo]!
	todo(id: Int!): Todo
	user(id: Int!): User
	users(ids: [Int!]!): [User]!
	group(id: Int!): Group
}

type Group struct {
	id: Int!
	kind: String!
	users: [User]! @hasRole(role: ADMIN)
}

type User {
	id: Int!
	name: String!
	age: Int!
	metadata: JSONString
	todos: [Todo]!
}

type Todo {
	id: Int!
	title: String!
	content: String!
	user: User
}

enum Role {
	ADMIN
	USER
}

directive @hasRole(role: Role!) on FIELD_DEFINITION
`

var page = []byte(`
<!DOCTYPE html>
<html>
	<head>
		<link href="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.11.11/graphiql.min.css" rel="stylesheet" />
		<script src="https://cdnjs.cloudflare.com/ajax/libs/es6-promise/4.1.1/es6-promise.auto.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/fetch/2.0.3/fetch.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react/16.2.0/umd/react.production.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/react-dom/16.2.0/umd/react-dom.production.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.11.11/graphiql.min.js"></script>
	</head>
	<body style="width: 100%; height: 100%; margin: 0; overflow: hidden;">
		<div id="graphiql" style="height: 100vh;">Loading...</div>
		<script>
			function graphQLFetcher(graphQLParams) {
				return fetch("/query", {
					method: "post",
					body: JSON.stringify(graphQLParams),
					credentials: "include",
				}).then(function (response) {
					return response.text();
				}).then(function (responseBody) {
					try {
						return JSON.parse(responseBody);
					} catch (error) {
						return responseBody;
					}
				});
			}
			ReactDOM.render(
				React.createElement(GraphiQL, {fetcher: graphQLFetcher}),
				document.getElementById("graphiql")
			);
		</script>
	</body>
</html>
`)

func query(tableName string, conditions squirrel.Sqlizer) squirrel.SelectBuilder {
	var query squirrel.SelectBuilder

	switch tableName {
	case "users":
		query = selector.Select("id", "name", "age", "isadmin", "metadata").From("users").Where(conditions)
	case "todos":
		query = selector.Select("id", "title", "content", "userid").From("todos").Where(conditions)
	default:
		log.Fatalf("unknown table name\n: %s", tableName)
	}

	str, args, err := query.ToSql()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(str, args)

	return query
}
