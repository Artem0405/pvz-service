// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import (
	context "context"

	domain "github.com/Artem0405/pvz-service/internal/domain"
	mock "github.com/stretchr/testify/mock"

	time "time"

	uuid "github.com/google/uuid"
)

// PVZRepository is an autogenerated mock type for the PVZRepository type
type PVZRepository struct {
	mock.Mock
}

// CreatePVZ provides a mock function with given fields: ctx, pvz
func (_m *PVZRepository) CreatePVZ(ctx context.Context, pvz domain.PVZ) (uuid.UUID, error) {
	ret := _m.Called(ctx, pvz)

	if len(ret) == 0 {
		panic("no return value specified for CreatePVZ")
	}

	var r0 uuid.UUID
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, domain.PVZ) (uuid.UUID, error)); ok {
		return rf(ctx, pvz)
	}
	if rf, ok := ret.Get(0).(func(context.Context, domain.PVZ) uuid.UUID); ok {
		r0 = rf(ctx, pvz)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(uuid.UUID)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, domain.PVZ) error); ok {
		r1 = rf(ctx, pvz)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAllPVZs provides a mock function with given fields: ctx
func (_m *PVZRepository) GetAllPVZs(ctx context.Context) ([]domain.PVZ, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetAllPVZs")
	}

	var r0 []domain.PVZ
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]domain.PVZ, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []domain.PVZ); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]domain.PVZ)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListPVZs provides a mock function with given fields: ctx, limit, afterRegistrationDate, afterID
func (_m *PVZRepository) ListPVZs(ctx context.Context, limit int, afterRegistrationDate *time.Time, afterID *uuid.UUID) ([]domain.PVZ, error) {
	ret := _m.Called(ctx, limit, afterRegistrationDate, afterID)

	if len(ret) == 0 {
		panic("no return value specified for ListPVZs")
	}

	var r0 []domain.PVZ
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int, *time.Time, *uuid.UUID) ([]domain.PVZ, error)); ok {
		return rf(ctx, limit, afterRegistrationDate, afterID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int, *time.Time, *uuid.UUID) []domain.PVZ); ok {
		r0 = rf(ctx, limit, afterRegistrationDate, afterID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]domain.PVZ)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int, *time.Time, *uuid.UUID) error); ok {
		r1 = rf(ctx, limit, afterRegistrationDate, afterID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewPVZRepository creates a new instance of PVZRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPVZRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *PVZRepository {
	mock := &PVZRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
