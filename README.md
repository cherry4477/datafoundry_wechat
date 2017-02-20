# datafoundry_wechat

```
datafoundry微信支付微服务
```

##数据库设计

```
CREATE TABLE IF NOT EXISTS DF_WECHATORDERS
(
    ID                BIGINT NOT NULL AUTO_INCREMENT,
    OUT_TRADE_NO      VARCHAR(64),
    NONCE_STR         VARCHAR(64),
    ORDERSIGN         VARCHAR(256),
    TOTAL_FEE         DOUBLE(10,2),
    TRADE_TYPE        VARCHAR(32),
    PREPAY_ID         VARCHAR(256),
    CODE_URL          VARCHAR(256),
    CASH_FEE          DOUBLE(10,2),
    FEE_TYPE          VARCHAR(32),
    BANK_TYPE         VARCHAR(32),
    OPENID            VARCHAR(256),
    TRANSACTION_ID    VARCHAR(256),
    TIME_END          VARCHAR(32),
    CREATE_AT         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UPDATE_AT         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    REGION            VARCHAR(32),
    USERNAME          VARCHAR(32),
    NAMESPACE         VARCHAR(64),
    STATUS            VARCHAR(32),
    PRIMARY KEY (ID)
) DEFAULT CHARSET=UTF8;
```

## API设计

### POST /charge/v1/wechat/recharge

生成一个

Body Parameters:
```
amount: 充值金额
namespace: 命名空间
```
eg:
```
POST /charge/v1/wechat/recharge HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
{
    "amount": 0.01,
	"namespace": "wangmeng5"
}
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.Code_url: 生成二维码url
data.Out_trade_no: 订单号
data.Total_fee: 订单金额
data.Trade_type: 订单类型
```
