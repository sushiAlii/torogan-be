package handlers

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	pb "github.com/sushiAlii/torogan-be/gen/uploadv1"
	"github.com/sushiAlii/torogan-be/pkg/interceptors"
	"github.com/sushiAlii/torogan-be/pkg/services"
)

type UploadHandler struct {
	uploadService *services.UploadService
}

func NewUploadHandler(s *services.UploadService) *UploadHandler {
	return &UploadHandler{
		uploadService: s,
	}
}

func (h *UploadHandler) CreatePresignedUpload(ctx context.Context, req *connect.Request[pb.CreatePresignedUploadRequest]) (*connect.Response[pb.CreatePresignedUploadResponse], error) {
	if _, err := interceptors.MustUserID(ctx); err != nil {
		return nil, err
	}

	uploadURL, publicURL, key, err := h.uploadService.CreatePresignedUpload(ctx, req.Msg.GetContentType(), req.Msg.GetFileExt())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create presigned upload: %w", err))
	}

	return connect.NewResponse(&pb.CreatePresignedUploadResponse{
		UploadUrl: uploadURL,
		PublicUrl: publicURL,
		Key:       key,
	}), nil
}
