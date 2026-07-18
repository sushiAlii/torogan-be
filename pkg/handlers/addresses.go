package handlers

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"gorm.io/gorm"

	pb "github.com/sushiAlii/torogan-be/gen/addressv1"
	"github.com/sushiAlii/torogan-be/internal/models"
	"github.com/sushiAlii/torogan-be/pkg/interceptors"
	"github.com/sushiAlii/torogan-be/pkg/services"
)

type AddressesHandler struct {
	addressesService *services.AddressService
}

func NewAddressesHandler(s *services.AddressService) *AddressesHandler {
	return &AddressesHandler{
		addressesService: s,
	}
}

func (h *AddressesHandler) CreateAddress(ctx context.Context, req *connect.Request[pb.CreateAddressRequest]) (*connect.Response[pb.Address], error) {
	msg := req.Msg

	if _, err := interceptors.MustUserID(ctx); err != nil {
		return nil, err
	}

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	createdAddress, err := h.addressesService.CreateAddress(models.Address{
		StreetAddress:   msg.GetStreetAddress(),
		ExtendedAddress: msg.GetExtendedAddress(),
		City:            msg.GetCity(),
		State:           msg.GetState(),
		CountryCode:     msg.GetCountryCode(),
		Latitude:        msg.GetLatitude(),
		Longitude:       msg.GetLongitude(),
		GooglePlaceID:   msg.GetGooglePlaceId(),
		PropertyID:      propertyUUID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(createdAddress)), nil
}

func (h *AddressesHandler) GetAddressByID(ctx context.Context, req *connect.Request[pb.GetAddressByIDRequest]) (*connect.Response[pb.Address], error) {
	msg := req.Msg

	addressUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid address ID: %w", err))
	}

	address, err := h.addressesService.GetAddressByID(addressUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("address not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(address)), nil
}

func (h *AddressesHandler) GetAddressByPropertyID(ctx context.Context, req *connect.Request[pb.GetAddressByPropertyIDRequest]) (*connect.Response[pb.Address], error) {
	msg := req.Msg

	propertyUUID, err := uuid.Parse(msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid property ID: %w", err))
	}

	address, err := h.addressesService.GetAddressByPropertyID(propertyUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("address not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(address)), nil
}

func (h *AddressesHandler) UpdateAddressByID(ctx context.Context, req *connect.Request[pb.UpdateAddressByIDRequest]) (*connect.Response[pb.Address], error) {
	msg := req.Msg

	if _, err := interceptors.MustUserID(ctx); err != nil {
		return nil, err
	}

	addressUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid address ID: %w", err))
	}

	updatedAddress, err := h.addressesService.UpdateAddressByID(models.Address{
		ID:              addressUUID,
		StreetAddress:   msg.GetStreetAddress(),
		ExtendedAddress: msg.GetExtendedAddress(),
		City:            msg.GetCity(),
		State:           msg.GetState(),
		CountryCode:     msg.GetCountryCode(),
		Latitude:        msg.GetLatitude(),
		Longitude:       msg.GetLongitude(),
		GooglePlaceID:   msg.GetGooglePlaceId(),
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("address not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(h.mapToProto(updatedAddress)), nil
}

func (h *AddressesHandler) DeleteAddressByID(ctx context.Context, req *connect.Request[pb.DeleteAddressByIDRequest]) (*connect.Response[pb.DeleteAddressByIDResponse], error) {
	msg := req.Msg

	if _, err := interceptors.MustUserID(ctx); err != nil {
		return nil, err
	}

	addressUUID, err := uuid.Parse(msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid address ID: %w", err))
	}

	if err := h.addressesService.DeleteAddressByID(addressUUID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("address not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.DeleteAddressByIDResponse{
		Success: true,
		Message: fmt.Sprintf("address with ID %s has been deleted", addressUUID.String()),
	}), nil
}

func (h *AddressesHandler) mapToProto(dbAddress *models.Address) *pb.Address {
	return &pb.Address{
		Id:              dbAddress.ID.String(),
		StreetAddress:   dbAddress.StreetAddress,
		ExtendedAddress: &dbAddress.ExtendedAddress,
		City:            dbAddress.City,
		State:           dbAddress.State,
		CountryCode:     dbAddress.CountryCode,
		Latitude:        dbAddress.Latitude,
		Longitude:       dbAddress.Longitude,
		GooglePlaceId:   &dbAddress.GooglePlaceID,
		PropertyId:      dbAddress.PropertyID.String(),
	}
}
