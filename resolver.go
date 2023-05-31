package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Masterminds/squirrel"
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
