package handlers

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/sushiAlii/torogan-be/gen/authv1"
	"github.com/sushiAlii/torogan-be/pkg/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(as *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: as,
	}
}

func (h *AuthHandler) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest],) (*connect.Response[authv1.RegisterResponse], error) {

	user, tokens, err := h.authService.Register(req.Msg.Email, req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	
	res := connect.NewResponse(&authv1.RegisterResponse{
		AccessToken: tokens.AccessToken,
		User: &authv1.User{
			Id:    user.ID.String(),
			Email: user.Email,
			AvatarUrl: user.AvatarURL,
		},
	})

	setRefreshCookie(res.Header(), tokens.RefreshToken)

	return res, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {

	user, token, err := h.authService.Login(req.Msg.Email, req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	res := connect.NewResponse(&authv1.LoginResponse{
		AccessToken: token.AccessToken,
		User: &authv1.User{
			Id:			user.ID.String(),
			Email:		user.Email,
			AvatarUrl: 	user.AvatarURL,
		},
	})

	setRefreshCookie(res.Header(), token.RefreshToken)

	return res, nil
}

func setRefreshCookie(header http.Header, token string) {
	cookie := &http.Cookie{
		Name:		"refresh_token",
		Value:		token,
		Path:		"/",
		MaxAge: 	int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: 	true,
		Secure:		true,
		SameSite: 	http.SameSiteStrictMode,
	}

	header.Add("Set-Cookie", cookie.String())
}