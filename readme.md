## AWS 服务器IP切换工具 使用方法

## 1.配置信息:

```json
{
  "instances": [
    {
      "name": "usa-f-ubuntu",
      "instance_name": "USA-F-Ubuntu-1-1",
      "static_ip_name": "USA-F-StaticIp-1"
    }
  ]
}
```

## 2.启动还需要配置环境变量:

```shell
# 设置 AWS 凭证
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1

# 设置端口（可选，默认8080）
export PORT=8080
```

## 3.检查服务状态:

```shell
➜  .aws curl "http://127.0.0.1:8080/health" | jq
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100    47  100    47    0     0  56969      0 --:--:-- --:--:-- --:--:-- 47000
{
  "message": "Service is running",
  "success": true
}
```

## 4.检查服务列表:

```shell
➜  .aws curl "http://127.0.0.1:8080/instances" | jq
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100   120  100   120    0     0   114k      0 --:--:-- --:--:-- --:--:--  117k
{
  "data": [
    {
      "name": "usa-f-ubuntu",
      "instance_name": "USA-F-Ubuntu-1-1",
      "static_ip_name": "USA-F-StaticIp-1"
    }
  ],
  "success": true
}
```

## 5.切换弹性IP:

```shell
➜  .aws curl "http://127.0.0.1:8080/change-ip?instance=usa-f-ubuntu" | jq
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100   135  100   135    0     0     11      0  0:00:12  0:00:11  0:00:01    30
{
  "instance_name": "USA-F-Ubuntu-1-1",
  "old_ip": "23.21.153.69",
  "new_ip": "44.202.35.19",
  "message": "IP changed successfully",
  "success": true
}
```