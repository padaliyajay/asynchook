# Asynchook
Asynchook allows you run task in background by creating a hook which call your url with specified payload. For example, when you want to send mail, you can make it in background by creating a hook which call your url with mail data. Your specified url will be called with the payload you provided. For that you need to send redis event as description below.

### Build
```bash
go build -o asynchook
```

### Run
```bash
./asynchook --config=/etc/asynchook/config.yaml
```

### Configuration
```yaml
redis:
  addr: localhost:6379
  db: 0
  password:
channels:
  - name: default
    ratelimit: 2/s # rate limit for this channel Ex. 2/s, 60/m, 300/h
```

### Usage
##### Send event to redis
```bash
HSET asynchook:1001 id 1001 url http://localhost:8080/mail payload '[YOUR JSON TEXT]' timestamp 1600000000 secret '[Your Secret]'
ZADD asynchooks:default 1 1001
```
Here Id and URL are mandatory fields. But payload, timestamp and secret are optional.

## License
MIT