docker build -t jiemu-batch-test .

export AWS_ACCESS_KEY_ID=
export AWS_SECRET_ACCESS_KEY=
export AWS_DEFAULT_REGION=

aws ecr get-login-password --region ap-northeast-2 | docker login --username AWS --password-stdin --

docker tag -- "--jiemu-batch-test:latest"
docker push "--jiemu-batch-test:latest"