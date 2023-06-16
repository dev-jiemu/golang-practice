## 2023.06.16 GoLang & AWS DynamoDB

Ref.
- https://www.joinc.co.kr/w/man/12/golang/dynamo
- https://blog.kico.co.kr/2022/03/17/aws-dynamodb
- https://docs.aws.amazon.com/sdk-for-go/api/
- https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/using-dynamodb-with-go-sdk.html

<br>

### 환경

- Golang 1.19
- AWS(도쿄 리전) ~~서울 리전에서 공부하다 사고칠것 같아서~~
- Mac

### Sample schema

- json/schema.json

### 개념정리

- PK : 단일 PK, 복합 PK 존재. 단일 PK는 반드시 한개만, 복합 PK는 partition key(1), sort key(2)
- 단일 PK는 hash type key로 사용, 기본값은 스칼라 데이터 형식만 허용
- 복합 PK는 partition key는 hash, sort key는 range 타입

### Hash type key & Range type key

- hash : equals(=) 검색
- range : like(%) 검색

<br>

### AWS CLI 명령어

- table create

```
aws dynamodb create-table --cli-input-json file://schema.json
```

- table scan
```
aws dynamodb scan --table-name jiemu_test_table
```

<br>

### DynamoDb에 저장하는 방식

- 일반 JSON 데이터

```json
{
  "id": "test",
  "name": "jiemu"
}
```

- DynamoDB로 들어갈때 데이터

```json
{
  "Item": {
    "id": {
      "S": "test"
    },
    "name": {
      "S": "jiemu"
    }
  }
}
```

aws-sdk-go module
- <strong>dynamodbattribute.MarshalMap</strong> : Json to AWS DynamoDB Map
- <strong>dynamodbattribute.Unmarshaling</strong> : AWS DynamoDB Map to Json



### 