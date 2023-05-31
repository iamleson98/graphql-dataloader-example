package main

import "context"

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
