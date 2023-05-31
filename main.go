package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	password = "anhyeuem98"
	database = "todos"
	username = "minh"
)

var (
	selector = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type MetadataModel struct {
	MetaData *JSONString `json:"meta_data"`
}

type ctxKey int

const (
	webCtx ctxKey = iota
	RoleCtx
)

func connectDB() (*sql.DB, error) {
	return sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, username, password, database))
}

func migrateDB(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil {
		return err
	}

	return nil
}

func todoByUserIDLoader(ctx context.Context, userIDs []int32) []*dataloader.Result[[]*Todo] {
	var (
		res     = make([]*dataloader.Result[[]*Todo], len(userIDs))
		todos   []*Todo
		todoMap = map[int32][]*Todo{}
	)

	db := ctx.Value(webCtx).(*resolver).db

	rows, err := query("todos", squirrel.Eq{"userid": userIDs}).RunWith(db).Query()
	if err != nil {
		goto errorLabel
	}
	defer rows.Close()

	for rows.Next() {
		var todo Todo
		err = rows.Scan(&todo.Id, &todo.Title, &todo.Content, &todo.UserID)
		if err != nil {
			goto errorLabel
		}

		todos = append(todos, &todo)
	}

	for _, td := range todos {
		todoMap[td.UserID] = append(todoMap[td.UserID], td)
	}

	for idx, id := range userIDs {
		res[idx] = &dataloader.Result[[]*Todo]{Data: todoMap[id]}
	}
	return res

errorLabel:
	for idx := range userIDs {
		res[idx] = &dataloader.Result[[]*Todo]{Error: err}
	}
	return res
}

func usersByIdLoader(ctx context.Context, keys []int32) []*dataloader.Result[*User] {
	db := ctx.Value(webCtx).(*resolver).db

	res := make([]*dataloader.Result[*User], len(keys))
	userMap := map[int32]*User{}

	rows, err := query("users", squirrel.Eq{"id": keys}).RunWith(db).Query()
	if err != nil {
		goto fatal
	}

	for rows.Next() {
		var u User
		err = rows.Scan(&u.Id, &u.Name, &u.Age)
		if err != nil {
			goto fatal
		}

		userMap[u.Id] = &u
	}

	for idx, id := range keys {
		res[idx] = &dataloader.Result[*User]{Data: userMap[id]}
	}

	return res

fatal:
	for idx := range keys {
		res[idx] = &dataloader.Result[*User]{
			Error: err,
		}
	}
	return res
}

var (
	UsersByIdLoader     = dataloader.NewBatchedLoader(usersByIdLoader, dataloader.WithBatchCapacity[int32, *User](200))
	TodosByUserIDLoader = dataloader.NewBatchedLoader(todoByUserIDLoader, dataloader.WithBatchCapacity[int32, []*Todo](200))
)

type Group struct {
	Id   int32  `json:"id"`
	Kind string `json:"kind"`
}

type HasRoleDirective struct {
	Role Role
}

var roles = map[int]Role{
	1: ADMIN,
	0: USER,
}

func (h *HasRoleDirective) ImplementsDirective() string {
	return "hasRole"
}

func (h *HasRoleDirective) Validate(ctx context.Context, _ interface{}) error {
	role := ctx.Value(RoleCtx).(Role)
	fmt.Println("------", role)
	if role == h.Role {
		return nil
	}

	return errors.New("you are not allowed")
}

type Role string

const (
	ADMIN Role = "ADMIN"
	USER  Role = "USER"
)

type resolver struct {
	db *sql.DB
}

func (r *resolver) Group(ctx context.Context, args struct{ Id int32 }) (*Group, error) {
	return &Group{
		Id:   1,
		Kind: "something",
	}, nil
}

func (r *resolver) Todos(ctx context.Context, args struct{ Ids []int32 }) ([]*Todo, error) {
	if len(args.Ids) == 0 {
		return []*Todo{}, nil
	}

	rows, err := query("todos", squirrel.Eq{"id": args.Ids}).RunWith(r.db).Query()
	if err != nil {
		return nil, err
	}

	var res []*Todo
	for rows.Next() {
		var td Todo
		err = rows.Scan(&td.Id, &td.Title, &td.Content, &td.UserID)
		if err != nil {
			return nil, err
		}

		res = append(res, &td)
	}

	return res, nil
}

func (r *resolver) Todo(ctx context.Context, args struct{ Id int32 }) (*Todo, error) {
	var res Todo
	err := query("todos", squirrel.Eq{"id": args.Id}).
		RunWith(r.db).
		QueryRow().
		Scan(&res.Id, &res.Title, &res.Content, &res.UserID)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (r *resolver) User(ctx context.Context, args struct{ Id int32 }) (*User, error) {
	var res User
	var meta []byte

	err := query("users", squirrel.Eq{"id": args.Id}).
		RunWith(r.db).
		QueryRow().
		Scan(&res.Id, &res.Name, &res.Age, &meta)
	if err != nil {
		return nil, err
	}

	if len(meta) > 0 {
		err = json.Unmarshal(meta, &res.MetaData)
		if err != nil {
			return nil, err
		}
	}

	return &res, nil
}

type User struct {
	Id   int32  `json:"id"`
	Name string `json:"name"`
	Age  int32  `json:"age"`
	MetadataModel
}

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

type Group {
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

func (u *User) Todos(ctx context.Context) ([]*Todo, error) {
	return TodosByUserIDLoader.Load(ctx, u.Id)()
}

type Todo struct {
	Id      int32  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	UserID  int32  `json:"user_id"`
}

func (t *Todo) User(ctx context.Context) (*User, error) {
	user, err := UsersByIdLoader.Load(ctx, t.UserID)()
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *resolver) Users(ctx context.Context, args struct{ Ids []int32 }) ([]*User, error) {
	rows, err := query("users", squirrel.Eq{"id": args.Ids}).RunWith(r.db).Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var res []*User

	for rows.Next() {
		var user User
		var meta []byte
		err = rows.Scan(&user.Id, &user.Name, &user.Age, &meta)
		if err != nil {
			return nil, err
		}

		if len(meta) > 0 {
			err = json.Unmarshal(meta, &user.MetaData)
			if err != nil {
				return nil, err
			}
		}

		res = append(res, &user)
	}

	return res, nil
}

func (g *Group) Users(ctx context.Context) ([]*User, error) {
	return []*User{
		{Id: 1, Name: "lol"},
	}, nil
}

func main() {
	db, err := connectDB()
	if err != nil {
		log.Fatalln(err)
	}
	err = migrateDB(db)
	if err != nil {
		if !strings.Contains(err.Error(), "no change") { // ignore no change error
			log.Fatalln(err)
		}
	}

	opts := []graphql.SchemaOpt{
		graphql.UseFieldResolvers(),
		graphql.MaxParallelism(20),
		graphql.Directives(&HasRoleDirective{}),
	}
	res := &resolver{db}

	schema := graphql.MustParseSchema(txt, res, opts...)

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(page)
	}))

	http.Handle("/query", middleWare(res, &relay.Handler{Schema: schema}))

	log.Fatal(http.ListenAndServe(":8000", nil))
}

func middleWare(res *resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		ctx = context.WithValue(ctx, webCtx, res)
		ctx = context.WithValue(ctx, RoleCtx, roles[rand.Intn(2)])

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
