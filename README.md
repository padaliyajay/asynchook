# Asynchook
Offload heavy task from the synchronous process to run later as web hook. 

Asynchook allows you run task in background by creating a hook which call your url with specified payload. For example, when you want to send mail, you can make it in background by creating a hook which call your url with mail data. Your specified url will be called with the payload you provided. For that you need to send redis event as description below.

### Use cases
- Sending email on user action but don't make your user wait for confirmation while waiting for smtp response. 
- Send bulk notification email with rate limit
- Call web hook with automatic retry on fail
- Run task in background

### Installation
```bash
wget https://github.com/padaliyajay/asynchook/releases/download/v1.1.0/asynchook_1.1.0_amd64.deb
dpkg -i asynchook_1.1.0_amd64.deb
systemctl enable asynchook
```

### Run
```bash
systemctl start asynchook
```

### Configuration
File: /etc/asynchook/config.yaml
```yaml
# Redis configuration
# Asynchook uses redis for message queueing
redis:
  addr: localhost:6379
  db: 0
  password:

# Asynchook error log
# logFile: /var/log/asynchook.log

# Asynchook channels
# You can add multiple channels with different ratelimits
channels:
  - name: default
    ratelimit: 2/s # rate limit for this channel Ex. 2/s, 60/m, 300/h
```

### Usage
##### Send event to redis
```bash
HSET asynchook:1001 id 1001 url http://localhost:8080/mail payload '[YOUR JSON TEXT]' secret '[Your Secret]' run_after_time '[UNIX TIMESTAMP]' expire_time '[UNIX TIMESTAMP]'
ZADD asynchooks:default 1 1001
```
Here Id and URL are mandatory fields. But payload, secret and others are optional.

## License
MIT
