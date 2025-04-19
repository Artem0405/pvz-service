package grpc

import (
	"context"
	"log/slog"

	"github.com/Artem0405/pvz-service/internal/repository"
	pb "github.com/Artem0405/pvz-service/pkg/pvz/v1" // Важно: pb - это ваш сгенерированный пакет
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PVZServer реализует интерфейс pvz_v1.PVZServiceServer
type PVZServer struct {
	pb.UnimplementedPVZServiceServer // Встраивание для совместимости

	pvzRepo repository.PVZRepository // Зависимость
}

// NewPVZServer конструктор
func NewPVZServer(pvzRepo repository.PVZRepository) *PVZServer {
	return &PVZServer{
		pvzRepo: pvzRepo,
	}
}

// GetPVZList реализация RPC метода
func (s *PVZServer) GetPVZList(ctx context.Context, req *pb.GetPVZListRequest) (*pb.GetPVZListResponse, error) {
	slog.InfoContext(ctx, "gRPC GetPVZList request received")

	// 1. Вызов репозитория
	domainPVZs, err := s.pvzRepo.GetAllPVZs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "gRPC: Failed to get all PVZs from repository", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to retrieve PVZ list: %v", err)
	}

	// 2. Конвертация Domain -> Protobuf
	protoPVZs := make([]*pb.PVZ, len(domainPVZs)) // Предварительно выделяем память
	for i, domainPVZ := range domainPVZs {
		protoPVZs[i] = &pb.PVZ{ // Создаем Protobuf сообщение
			Id:               domainPVZ.ID.String(),                       // uuid.UUID -> string
			RegistrationDate: timestamppb.New(domainPVZ.RegistrationDate), // time.Time -> timestamppb.Timestamp
			City:             domainPVZ.City,                              // string -> string
		}
	}

	slog.InfoContext(ctx, "gRPC: Successfully retrieved and converted PVZ list", "count", len(protoPVZs))

	// 3. Формирование и возврат ответа
	response := &pb.GetPVZListResponse{
		Pvzs: protoPVZs, // Вставляем сконвертированный срез
	}
	return response, nil
}
