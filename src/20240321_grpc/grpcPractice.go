package main

import (
	"context"
	pb "example/20240321_grpc/sample/config"
	//"google.golang.org/grpc"
	"log"
	//"net"
)

type server struct {
	//pb.UnimplementedConfigStoreServer
}

// gPRC .proto 파일 생성하고 만들고 대강 사용하는 코드
// Ref. https://lejewk.github.io/grpc-go-example, https://grpc.io/docs/languages/go/quickstart

func (s *server) Get(ctx context.Context, in *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	log.Printf("Received profile: %v", in.GetProfile())
	return &pb.ConfigResponse{JsonConfig: `"{"main":"http://google.com"}"`}, nil
}

func main() {
	//lis, err := net.Listen("tcp", ":8088")
	//if err != nil {
	//	log.Fatalf("failed to listen: %v", err)
	//}
	//
	//s := grpc.NewServer()
	//pb.RegisterConfigStoreServer(s, &server{})
	//if err := s.Serve(lis); err != nil {
	//	log.Fatalf("failed to serve: %v", err)
	//}
}
