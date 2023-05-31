package main

import "context"

type User struct {
	Id   int32  `json:"id"`
	Name string `json:"name"`
	Age  int32  `json:"age"`
	MetadataModel
}

func (u *User) Todos(ctx context.Context) ([]*Todo, error) {
	return TodosByUserIDLoader.Load(ctx, u.Id)()
}
