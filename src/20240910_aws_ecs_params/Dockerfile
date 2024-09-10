FROM golang:1.22.2-alpine

RUN apk --no-cache add tzdata && \
	cp /usr/share/zoneinfo/Asia/Seoul /etc/localtime && \
	echo "Asia/Seoul" > /etc/timezone && \
	apk del tzdata
RUN apk add libc6-compat

WORKDIR /app
CMD ["./ecs_container_override"]