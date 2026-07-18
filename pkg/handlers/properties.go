package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"gorm.io/gorm"

	featurev1 "github.com/sushiAlii/torogan-be/gen/featurev1"
	pb "github.com/sushiAlii/torogan-be/gen/propertyv1"
	"github.com/sushiAlii/torogan-be/internal/models"
	"github.com/sushiAlii/torogan-be/pkg/services"
)

type PropertiesHandler struct {
	propertiesService *services.PropertyService
}

func NewPropertiesHandler(s *services.PropertyService) *PropertiesHandler {
	return &PropertiesHandler{
		propertiesService: s,
	}
}

func (h *PropertiesHandler) CreateProperty(ctx context.Context, req *connect.Request[pb.CreatePropertyRequest]) (*connect.Response[pb.Property], error) {
	msg := req.Msg

	ownerUUID, err := uuid.Parse(msg.GetOwnerId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid owner UUID: %w", err))
	}

	priceFloat, err := strconv.ParseFloat(msg.GetPrice(), 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid price format: %w", err))
	}

	newProperty := models.Property{
		Title:       msg.GetTitle(),
		Description: msg.GetDescription(),
		SizeSqM:     msg.GetSizeSqM(),
		Bedrooms:    msg.GetBedrooms(),
		Bathrooms:   msg.GetBathrooms(),
		Price:       priceFloat,
		OwnerID:     ownerUUID,
	}

	createdProperty, err := h.propertiesService.CreateProperty(newProperty)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(createdProperty, "")), nil
}

func (h *PropertiesHandler) GetPropertyByID(ctx context.Context, req *connect.Request[pb.GetPropertyByIDRequest]) (*connect.Response[pb.Property], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	property, err := h.propertiesService.GetPropertyByID(propertyUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("property not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	images, err := h.propertiesService.ListPropertyImages(propertyUUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	mainImageURL := ""
	for _, img := range images {
		if img.IsMain {
			mainImageURL = img.URL
			break
		}
	}

	protoProperty := h.mapToProto(property, mainImageURL)
	protoProperty.Images = h.mapImagesToProto(images)

	return connect.NewResponse(protoProperty), nil
}

func (h *PropertiesHandler) GetPropertyList(ctx context.Context, req *connect.Request[pb.GetPropertyListRequest]) (*connect.Response[pb.GetPropertyListResponse], error) {
	msg := req.Msg
	search := msg.GetSearch()

	limit := int(msg.GetLimit())
	if limit <= 0 {
		limit = 25
	}

	var cursorUUID uuid.UUID
	if cursor := msg.GetCursor(); cursor != "" {
		cUUID, err := uuid.Parse(cursor)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid cursor UUID: %w", err))
		}
		cursorUUID = cUUID
	}

	properties, totalCount, err := h.propertiesService.GetPropertyList(search, limit, cursorUUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	propertyIDs := make([]uuid.UUID, len(properties))
	for i, p := range properties {
		propertyIDs[i] = p.ID
	}

	mainImageURLs, err := h.propertiesService.GetMainImageURLs(propertyIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoProperties := make([]*pb.Property, len(properties))
	for i, p := range properties {
		protoProperties[i] = h.mapToProto(&p, mainImageURLs[p.ID])
	}

	nextCursor := ""
	if len(properties) == limit {
		nextCursor = properties[len(properties)-1].ID.String()
	}

	return connect.NewResponse(&pb.GetPropertyListResponse{
		Properties: protoProperties,
		NextCursor: nextCursor,
		TotalCount: int32(totalCount),
	}), nil
}

func (h *PropertiesHandler) UpdatePropertyByID(ctx context.Context, req *connect.Request[pb.UpdatePropertyByIDRequest]) (*connect.Response[pb.Property], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	priceFloat, err := strconv.ParseFloat(msg.GetPrice(), 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid price format: %w", err))
	}

	updatedProperty, err := h.propertiesService.UpdatePropertyByID(models.Property{
		ID:          propertyUUID,
		Title:       msg.GetTitle(),
		SizeSqM:     msg.GetSizeSqM(),
		Description: msg.GetDescription(),
		Bedrooms:    msg.GetBedrooms(),
		Bathrooms:   msg.GetBathrooms(),
		Price:       priceFloat,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("property not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	mainImageURLs, err := h.propertiesService.GetMainImageURLs([]uuid.UUID{updatedProperty.ID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(updatedProperty, mainImageURLs[updatedProperty.ID])), nil
}

func (h *PropertiesHandler) DeletePropertyByID(ctx context.Context, req *connect.Request[pb.DeletePropertyByIDRequest]) (*connect.Response[pb.DeletePropertyByIDResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	if err := h.propertiesService.DeletePropertyByID(propertyUUID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("property not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.DeletePropertyByIDResponse{
		Success: true,
		Message: fmt.Sprintf("property with ID %s has been deleted", propertyUUID.String()),
	}), nil
}

func (h *PropertiesHandler) AddPropertyFeature(ctx context.Context, req *connect.Request[pb.AddPropertyFeatureRequest]) (*connect.Response[pb.ListPropertyFeaturesResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	if err := h.propertiesService.AddPropertyFeature(propertyUUID, uint(msg.GetFeatureId())); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	features, err := h.propertiesService.ListPropertyFeatures(propertyUUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ListPropertyFeaturesResponse{
		Features: h.mapFeaturesToProto(features),
	}), nil
}

func (h *PropertiesHandler) RemovePropertyFeature(ctx context.Context, req *connect.Request[pb.RemovePropertyFeatureRequest]) (*connect.Response[pb.DeletePropertyByIDResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	if err := h.propertiesService.RemovePropertyFeature(propertyUUID, uint(msg.GetFeatureId())); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("property feature not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.DeletePropertyByIDResponse{
		Success: true,
		Message: "feature has been removed from property",
	}), nil
}

func (h *PropertiesHandler) ListPropertyFeatures(ctx context.Context, req *connect.Request[pb.ListPropertyFeaturesRequest]) (*connect.Response[pb.ListPropertyFeaturesResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	features, err := h.propertiesService.ListPropertyFeatures(propertyUUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ListPropertyFeaturesResponse{
		Features: h.mapFeaturesToProto(features),
	}), nil
}

func (h *PropertiesHandler) AddPropertyImage(ctx context.Context, req *connect.Request[pb.AddPropertyImageRequest]) (*connect.Response[pb.ListPropertyImagesResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	images, err := h.propertiesService.AddPropertyImage(propertyUUID, msg.GetUrl(), msg.GetIsMain(), msg.GetPosition())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("property not found"))
		}
		if errors.Is(err, services.ErrMaxPropertyImages) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ListPropertyImagesResponse{
		Images: h.mapImagesToProto(images),
	}), nil
}

func (h *PropertiesHandler) RemovePropertyImage(ctx context.Context, req *connect.Request[pb.RemovePropertyImageRequest]) (*connect.Response[pb.DeletePropertyByIDResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	imageUUID, err := uuid.Parse(msg.GetImageId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid image ID: %w", err))
	}

	if err := h.propertiesService.RemovePropertyImage(propertyUUID, imageUUID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("property image not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.DeletePropertyByIDResponse{
		Success: true,
		Message: "image has been removed from property",
	}), nil
}

func (h *PropertiesHandler) ListPropertyImages(ctx context.Context, req *connect.Request[pb.ListPropertyImagesRequest]) (*connect.Response[pb.ListPropertyImagesResponse], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	images, err := h.propertiesService.ListPropertyImages(propertyUUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ListPropertyImagesResponse{
		Images: h.mapImagesToProto(images),
	}), nil
}

func (h *PropertiesHandler) mapImagesToProto(dbImages []models.PropertyImage) []*pb.PropertyImage {
	protoImages := make([]*pb.PropertyImage, len(dbImages))
	for i, img := range dbImages {
		protoImages[i] = &pb.PropertyImage{
			Id:         img.ID.String(),
			PropertyId: img.PropertyID.String(),
			Url:        img.URL,
			IsMain:     img.IsMain,
			Position:   img.Position,
		}
	}

	return protoImages
}

func (h *PropertiesHandler) mapFeaturesToProto(dbFeatures []models.Feature) []*featurev1.Feature {
	protoFeatures := make([]*featurev1.Feature, len(dbFeatures))
	for i, f := range dbFeatures {
		protoFeatures[i] = &featurev1.Feature{
			Id:   int32(f.ID),
			Name: f.Name,
		}
	}

	return protoFeatures
}

func (h *PropertiesHandler) mapToProto(dbProp *models.Property, mainImageURL string) *pb.Property {
	return &pb.Property{
		Id:           dbProp.ID.String(),
		Title:        dbProp.Title,
		SizeSqM:      dbProp.SizeSqM,
		Description:  dbProp.Description,
		Bedrooms:     dbProp.Bedrooms,
		Bathrooms:    dbProp.Bathrooms,
		Price:        fmt.Sprintf("%.2f", dbProp.Price),
		OwnerId:      dbProp.OwnerID.String(),
		MainImageUrl: mainImageURL,
	}
}
