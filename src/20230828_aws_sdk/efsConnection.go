package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"log"
)

// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/efs#Client.DescribeMountTargets
func main() {

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("ap-northeast-2"),
	)

	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := efs.NewFromConfig(cfg)
	mountTargetsResp, err := client.DescribeMountTargets(context.TODO(), &efs.DescribeMountTargetsInput{
		FileSystemId: aws.String("fs-________________"),
	})
	if err != nil {
		log.Fatalf("DescribeMountTargets : %v", err)
	}

	if len(mountTargetsResp.MountTargets) == 0 {
		log.Fatalf("MountTargets: no mount targets found for the given file system")
	}

	mountTargetIP := aws.ToString(mountTargetsResp.MountTargets[0].IpAddress) // 일단 target ip 까진 접근 완료임

	fmt.Println("mountTargetIp : ", mountTargetIP)

	// nfs 2049 port로 접근하라고?

}
