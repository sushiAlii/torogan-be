package handlers

import (
	"context"
	"errors"
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

func (h *AuthHandler) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest]) (*connect.Response[authv1.RegisterResponse], error) {

	user, role, tokens, err := h.authService.Register(req.Msg.Email, req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	res := connect.NewResponse(&authv1.RegisterResponse{
		AccessToken: tokens.AccessToken,
		User: &authv1.User{
			Id:        user.ID.String(),
			Email:     user.Email,
			AvatarUrl: user.AvatarURL,
			Role:      role,
		},
	})

	setRefreshCookie(res.Header(), tokens.RefreshToken)

	return res, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {

	user, role, tokens, err := h.authService.Login(req.Msg.Email, req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	res := connect.NewResponse(&authv1.LoginResponse{
		AccessToken: tokens.AccessToken,
		User: &authv1.User{
			Id:        user.ID.String(),
			Email:     user.Email,
			AvatarUrl: user.AvatarURL,
			Role:      role,
		},
	})

	setRefreshCookie(res.Header(), tokens.RefreshToken)

	return res, nil
}

func (h *AuthHandler) SignInWithGoogle(ctx context.Context, req *connect.Request[authv1.SignInWithGoogleRequest]) (*connect.Response[authv1.SignInWithGoogleResponse], error) {

	user, role, tokens, err := h.authService.SignInWithGoogle(ctx, req.Msg.GetIdToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	res := connect.NewResponse(&authv1.SignInWithGoogleResponse{
		AccessToken: tokens.AccessToken,
		User: &authv1.User{
			Id:        user.ID.String(),
			Email:     user.Email,
			AvatarUrl: user.AvatarURL,
			Role:      role,
		},
	})

	setRefreshCookie(res.Header(), tokens.RefreshToken)

	return res, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *connect.Request[authv1.RefreshTokenRequest]) (*connect.Response[authv1.RefreshTokenResponse], error) {

	cookie, err := (&http.Request{Header: req.Header()}).Cookie("refresh_token")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing refresh token"))
	}

	userID, role, err := h.authService.ValidateRefreshToken(cookie.Value)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	user, resolvedRole, err := h.authService.GetUserByID(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	accessToken, err := h.authService.GenerateAccessToken(userID, role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&authv1.RefreshTokenResponse{
		AccessToken: accessToken,
		User: &authv1.User{
			Id:        user.ID.String(),
			Email:     user.Email,
			AvatarUrl: user.AvatarURL,
			Role:      resolvedRole,
		},
	}), nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	res := connect.NewResponse(&authv1.LogoutResponse{
		Success: true,
	})

	clearRefreshCookie(res.Header())

	return res, nil
}

func setRefreshCookie(header http.Header, token string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}

	header.Add("Set-Cookie", cookie.String())
}

func clearRefreshCookie(header http.Header) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}

	header.Add("Set-Cookie", cookie.String())
}
