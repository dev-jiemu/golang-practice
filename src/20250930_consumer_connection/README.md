#### 테스트 관련 모음 
```shell
# 모든 테스트 실행
go test -v

# 특정 테스트만 실행
go test -v -run TestBasicMessageConsumption

# 벤치마크 실행
go test -bench=. -benchmem

# 테스트 커버리지 확인
go test -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

```shell
# 테스트 메세지 발행
for i in {1..10}; do
  curl -u guest:guest -H "content-type:application/json" \
    -X POST http://localhost:15672/api/exchanges/%2f/amq.default/publish \
    -d "{\"properties\":{},\"routing_key\":\"jiemu-worker\",\"payload\":\"test message $i\",\"payload_encoding\":\"string\"}"
done
```