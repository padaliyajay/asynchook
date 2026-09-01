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
Each channel is paced evenly across its window rather than allowed to burst, so
`60/m` sends one hook every second instead of 60 at once.

### Usage
##### Send event to redis
```bash
HSET asynchook:1001 id 1001 url http://localhost:8080/mail payload '[YOUR JSON TEXT]' secret '[Your Secret]' run_after_time '[UNIX TIMESTAMP]' expire_time '[UNIX TIMESTAMP]'
ZADD asynchooks:default 1 1001
```
Here Id and URL are mandatory fields. But payload, secret and others are optional.
The score you pass to `ZADD` is the hook's priority (lower runs first) and is
preserved across scheduling and retries.

A hook that fails is retried up to 5 times with an exponential delay (5m, 15m,
45m, 2h15m, then 6h45m) before it is dropped. Each delivery is given up to one
minute to respond. Set `expire_time` on a hook to stop retrying past a deadline.

### Build from source
Requires Go 1.22 or newer. `build.sh` covers every build task:

```bash
./build.sh              # check, then build -> bin/asynchook
./build.sh build        # build only
./build.sh run          # build and run against config.local.yaml
./build.sh check        # gofmt + go vet + go test
./build.sh release      # cross-compiled tarballs (linux/darwin, amd64/arm64) -> dist/
./build.sh deb [arch]   # Debian package, amd64 by default or arm64 -> dist/
./build.sh clean        # remove bin/ and dist/
./build.sh help
```

`./build.sh run` expects a `config.local.yaml` pointing at your development
redis - copy `config.yaml` and edit it. It is gitignored.

Version numbers on release artifacts come from `git describe`, so tag the commit
before cutting a release. Building the `.deb` needs `dpkg-deb`
(`apt install dpkg` on Debian/Ubuntu, `brew install dpkg` on macOS).

## License
MIT
