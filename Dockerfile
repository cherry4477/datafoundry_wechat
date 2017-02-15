FROM golang:1.7.4

EXPOSE 8574

ENV TIME_ZONE=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TIME_ZONE /etc/localtime && echo $TIME_ZONE > /etc/timezone

COPY . /go/src/github.com/asiainfoLDP/datafoundry_wechat

WORKDIR /go/src/github.com/asiainfoLDP/datafoundry_wechat

RUN go build

CMD ["sh", "-c", "./datafoundry_wechat"]