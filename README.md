

### DEMO

```
```
ssh dev.maluta.com.br
```

```

### BUILD

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o <app name> .

For example:

```
```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o maluta-ssh .
```
```



