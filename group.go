package main

import (
	"context"
	"errors"
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

func (g *Group) Users(ctx context.Context) ([]*User, error) {
	return []*User{
		{Id: 1, Name: "lol"},
	}, nil
}
