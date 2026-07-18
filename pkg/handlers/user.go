package handlers

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"gorm.io/gorm"

	authv1 "github.com/sushiAlii/torogan-be/gen/authv1"
	pb "github.com/sushiAlii/torogan-be/gen/userv1"
	"github.com/sushiAlii/torogan-be/internal/models"
	"github.com/sushiAlii/torogan-be/pkg/interceptors"
	"github.com/sushiAlii/torogan-be/pkg/services"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(s *services.UserService) *UserHandler {
	return &UserHandler{
		userService: s,
	}
}

func (h *UserHandler) GetMe(ctx context.Context, req *connect.Request[pb.GetMeRequest]) (*connect.Response[authv1.User], error) {
	callerID, err := interceptors.MustUserID(ctx)
	if err != nil {
		return nil, err
	}

	userUUID, err := uuid.Parse(callerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid caller UUID: %w", err))
	}

	user, role, err := h.userService.GetByID(userUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(mapUserToProto(user, role)), nil
}

func (h *UserHandler) UpdateMe(ctx context.Context, req *connect.Request[pb.UpdateMeRequest]) (*connect.Response[authv1.User], error) {
	callerID, err := interceptors.MustUserID(ctx)
	if err != nil {
		return nil, err
	}

	userUUID, err := uuid.Parse(callerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid caller UUID: %w", err))
	}

	user, role, err := h.userService.UpdateContact(userUUID, req.Msg.GetName(), req.Msg.GetPhone())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(mapUserToProto(user, role)), nil
}

func mapUserToProto(user *models.User, role string) *authv1.User {
	return &authv1.User{
		Id:        user.ID.String(),
		Email:     user.Email,
		AvatarUrl: user.AvatarURL,
		Role:      role,
		Name:      user.Name,
		Phone:     user.Phone,
	}
}
