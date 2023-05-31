package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
