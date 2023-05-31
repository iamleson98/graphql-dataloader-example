package main

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/graph-gophers/dataloader/v7"
)

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
