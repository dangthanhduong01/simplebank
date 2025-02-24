package gapi

import (
	"fmt"

	db "github.com/dangthanhduong01/simplebank/db/sqlc"
	"github.com/dangthanhduong01/simplebank/db/utils"
	"github.com/dangthanhduong01/simplebank/pb"
	"github.com/dangthanhduong01/simplebank/token"
	"github.com/dangthanhduong01/simplebank/worker"
)

// Serves gRPC requests for our banking service
type Server struct {
	pb.UnimplementedSimpleBankServer
	config          utils.Config
	store           db.Store
	tokenMaker      token.Maker
	taskDistributor worker.TaskDistributor
}

// NewServer creates a new gRPC server.
func NewServer(config utils.Config, store db.Store, taskDistributor worker.TaskDistributor) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("Cannot create token maker: %w", err)
	}
	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		taskDistributor: taskDistributor,
	}

	return server, nil
}
