package services

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"gorm.io/gorm"

	pb "github.com/sushiAlii/torogan-be/gen/propertyv1"
	"github.com/sushiAlii/torogan-be/internal/models"
)

type PropertyService struct {
	DB *gorm.DB
}

func NewPropertyService(db *gorm.DB) *PropertyService {
	return &PropertyService{DB: db}
}

func (ps *PropertyService) CreateProperty(ctx context.Context, req *connect.Request[pb.CreatePropertyRequest]) (*connect.Response[pb.Property], error) {
	
	msg := req.Msg

	ownerUUID, err := uuid.Parse(msg.GetOwnerId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid owner UUID: %w", err))
	}

	priceFloat, err := strconv.ParseFloat(msg.GetPrice(), 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid price format: %w", err))
	}

	dbProperty := models.Property{
		Title:       msg.GetTitle(),
		SizeSqM:     msg.GetSizeSqM(),
		Description: msg.GetDescription(),
		Price:       priceFloat,
		OwnerID:     ownerUUID,
	}

	if err := ps.DB.WithContext(ctx).Create(&dbProperty).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(ps.mapToProto(&dbProperty)), nil
}

func (ps *PropertyService) GetPropertyByID(ctx context.Context, req *connect.Request[pb.GetPropertyByIDRequest]) (*connect.Response[pb.Property], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid property UUID: %w", err))
	}

	var dbProperty models.Property
	if err := ps.DB.WithContext(ctx).First(&dbProperty, "id = ?", propertyUUID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Property not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ps.mapToProto(&dbProperty)), nil
}

func (ps *PropertyService) GetPropertyList(ctx context.Context, req *connect.Request[pb.GetPropertyListRequest]) (*connect.Response[pb.GetPropertyListResponse], error) {

	msg := req.Msg

	limit := int(msg.GetLimit())
	if limit <= 0 {
		limit = 25
	}

	query := ps.DB.WithContext(ctx).Model(&models.Property{})
	if search := msg.GetSearch(); search != "" {
		query = query.Where("title ILIKE ?", "%"+search+"%")
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	query = query.Order("id ASC").Limit(limit)
	if cursor := msg.GetCursor(); cursor != "" {
		cursorUUID, err := uuid.Parse(cursor)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid cursor UUID: %w", err))
		}
		query = query.Where("id > ?", cursorUUID)
	}

	var dbProperties []models.Property
	if err := query.Find(&dbProperties).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoProperties := make([]*pb.Property, len(dbProperties))
	for i, dbProp := range dbProperties {
		protoProperties[i] = ps.mapToProto(&dbProp)
	}

	nextCursor := ""
	if len(dbProperties) == limit {
		nextCursor = dbProperties[len(dbProperties)-1].ID.String()
	}

	return connect.NewResponse(&pb.GetPropertyListResponse{
		Properties: protoProperties,
		NextCursor: nextCursor,
		TotalCount: int32(totalCount),
	}), nil
}

func (ps *PropertyService) UpdatePropertyByID(ctx context.Context, req *connect.Request[pb.UpdatePropertyByIDRequest]) (*connect.Response[pb.Property], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid property UUID: %w", err))
	}

	priceFloat, err := strconv.ParseFloat(msg.GetPrice(), 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid price format: %w", err))
	}

	var dbProperty models.Property
	if err := ps.DB.WithContext(ctx).First(&dbProperty, "id = ?", propertyUUID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Property not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	dbProperty.Title = msg.GetTitle()
	dbProperty.SizeSqM = msg.GetSizeSqM()
	dbProperty.Description = msg.GetDescription()
	dbProperty.Price = priceFloat

	if err := ps.DB.WithContext(ctx).Save(&dbProperty).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(ps.mapToProto(&dbProperty)), nil

}

func (ps *PropertyService) DeletePropertyByID(ctx context.Context, req *connect.Request[pb.DeletePropertyByIDRequest]) (*connect.Response[pb.DeletePropertyByIDResponse], error) {

	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Invalid property UUID: %w", err))
	}

	result := ps.DB.WithContext(ctx).Delete(&models.Property{}, "id = ?", propertyUUID)
	if result.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("Property not found"))
	}

	return connect.NewResponse(&pb.DeletePropertyByIDResponse{
		Success: true,
		Message: fmt.Sprintf("Property with ID %s has been deleted.", propertyUUID.String()),
	}), nil
}

func (s *PropertyService) mapToProto(dbProp *models.Property) *pb.Property {
	return &pb.Property{
		Id:          dbProp.ID.String(),
		Title:       dbProp.Title,
		SizeSqM:     dbProp.SizeSqM,
		Description: dbProp.Description,
		Price:       fmt.Sprintf("%.2f", dbProp.Price), 
		OwnerId:     dbProp.OwnerID.String(),
	}
}