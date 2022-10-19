package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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

type User struct {
	Id   int32  `json:"id"`
	Name string `json:"name"`
	Age  int32  `json:"age"`
}

type Todo struct {
	Id      int32  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	UserID  int32  `json:"user_id"`
}

type ctxKey int

const (
	webCtx ctxKey = iota
	dataloaderCtx
)

type resolver struct {
	db *sql.DB
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
	err := query("todos", squirrel.Eq{"id": args.Id}).RunWith(r.db).QueryRow().Scan(&res.Id, &res.Title, &res.Content, &res.UserID)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (t *Todo) User(ctx context.Context) (*User, error) {
	user, err := dataloaders_.userLoaders.Load(ctx, t.UserID)()
	if err != nil {
		return nil, err
	}

	return user, nil
}

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

const txt = `
schema {
	query: Query
}

type Query {}

extend type Query {
	todos(ids: [Int!]!): [Todo]!
	todo(id: Int!): Todo
}

type User {
	id: Int!
	name: String!
	age: Int!
}

type Todo {
	id: Int!
	title: String!
	content: String!
	user: User
}`

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
	}
	res := &resolver{db}

	schema := graphql.MustParseSchema(txt, res, opts...)

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(page)
	}))

	http.Handle("/query", middleWare(res, &relay.Handler{Schema: schema}))

	log.Fatal(http.ListenAndServe(":8000", nil))
}

type Dataloaders struct {
	userLoaders *dataloader.Loader[int32, *User]
}

func query(tableName string, cons squirrel.Sqlizer) squirrel.SelectBuilder {
	var query squirrel.SelectBuilder

	switch tableName {
	case "users":
		query = selector.Select("id", "name", "age").From("users").Where(cons)
	case "todos":
		query = selector.Select("id", "title", "content", "userid").From("todos").Where(cons)
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

func newDataLoaders() *Dataloaders {
	return &Dataloaders{
		userLoaders: dataloader.NewBatchedLoader(func(ctx context.Context, keys []int32) []*dataloader.Result[*User] {
			db := ctx.Value(webCtx).(*resolver).db

			res := []*dataloader.Result[*User]{}

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

				res = append(res, &dataloader.Result[*User]{
					Data: &u,
				})
			}

			return res

		fatal:
			for range keys {
				res = append(res, &dataloader.Result[*User]{
					Error: err,
				})
			}
			return res
		}),
	}
}

var dataloaders_ = newDataLoaders()

func middleWare(res *resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		ctx = context.WithValue(ctx, webCtx, res)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
