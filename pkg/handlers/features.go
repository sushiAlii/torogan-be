package handlers

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	pb "github.com/sushiAlii/torogan-be/gen/featurev1"
	"github.com/sushiAlii/torogan-be/internal/models"
	"github.com/sushiAlii/torogan-be/pkg/services"
)

type FeaturesHandler struct {
	featuresService *services.FeatureService
}

func NewFeaturesHandler(s *services.FeatureService) *FeaturesHandler {
	return &FeaturesHandler{
		featuresService: s,
	}
}

func (h *FeaturesHandler) CreateFeature(ctx context.Context, req *connect.Request[pb.CreateFeatureRequest]) (*connect.Response[pb.Feature], error) {
	msg := req.Msg

	createdFeature, err := h.featuresService.CreateFeature(models.Feature{
		Name: msg.GetName(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(createdFeature)), nil
}

func (h *FeaturesHandler) GetFeatureByID(ctx context.Context, req *connect.Request[pb.GetFeatureByIDRequest]) (*connect.Response[pb.Feature], error) {
	msg := req.Msg

	feature, err := h.featuresService.GetFeatureByID(uint(msg.GetId()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("feature not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(feature)), nil
}

func (h *FeaturesHandler) ListFeatures(ctx context.Context, req *connect.Request[pb.ListFeaturesRequest]) (*connect.Response[pb.ListFeaturesResponse], error) {
	msg := req.Msg

	limit := int(msg.GetLimit())
	if limit <= 0 {
		limit = 25
	}

	features, totalCount, err := h.featuresService.ListFeatures(msg.GetSearch(), limit, uint(msg.GetCursor()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoFeatures := make([]*pb.Feature, len(features))
	for i, f := range features {
		protoFeatures[i] = h.mapToProto(&f)
	}

	var nextCursor int32
	if len(features) == limit {
		nextCursor = int32(features[len(features)-1].ID)
	}

	return connect.NewResponse(&pb.ListFeaturesResponse{
		Features:   protoFeatures,
		NextCursor: nextCursor,
		TotalCount: int32(totalCount),
	}), nil
}

func (h *FeaturesHandler) UpdateFeatureByID(ctx context.Context, req *connect.Request[pb.UpdateFeatureByIDRequest]) (*connect.Response[pb.Feature], error) {
	msg := req.Msg

	updatedFeature, err := h.featuresService.UpdateFeatureByID(models.Feature{
		ID:   uint(msg.GetId()),
		Name: msg.GetName(),
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("feature not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(updatedFeature)), nil
}

func (h *FeaturesHandler) DeleteFeatureByID(ctx context.Context, req *connect.Request[pb.DeleteFeatureByIDRequest]) (*connect.Response[pb.DeleteFeatureByIDResponse], error) {
	msg := req.Msg

	if err := h.featuresService.DeleteFeatureByID(uint(msg.GetId())); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("feature not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.DeleteFeatureByIDResponse{
		Success: true,
		Message: fmt.Sprintf("feature with ID %d has been deleted", msg.GetId()),
	}), nil
}

func (h *FeaturesHandler) mapToProto(dbFeature *models.Feature) *pb.Feature {
	return &pb.Feature{
		Id:   int32(dbFeature.ID),
		Name: dbFeature.Name,
	}
}
