package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/asiainfoLDP/datafoundry_wechat/common"
	"github.com/asiainfoLDP/datafoundry_wechat/models"
	"github.com/asiainfoLDP/datafoundry_wechat/openshift"
	userapi "github.com/openshift/origin/pkg/user/api/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"net"
	"log"
	"io/ioutil"
	//"io"
	"bytes"
)

const (
	RemoteAddr = "dev.dataos.io:8443"

	DfRegion_CnNorth01 = "cn-north-1"
	DfRegion_CnNorth02 = "cn-north-2"

	NumDfRegions = 2
)

//=================================================
//get remote endpoint
//=================================================

var (
	osAdminClients map[string]*openshift.OpenshiftClient

	RechargeSercice string
	DataFoundryHost string
)

func BuildDataFoundryClient(infoEnv string, durPhase time.Duration) *openshift.OpenshiftClient {
	info := os.Getenv(infoEnv)
	params := strings.Split(strings.TrimSpace(info), " ")
	if len(params) != 3 {
		logger.Emergency("BuildDataFoundryClient, len(params) is not correct: ", len(params))
	}

	return openshift.CreateOpenshiftClient(infoEnv, params[0], params[1], params[2], durPhase)
}

func BuildServiceUrlPrefixFromEnv(name string, isHttps bool, addrEnv string, portEnv string) string {
	var addr string
	if models.SetPlatform {
		addr = RemoteAddr
	} else {
		addr = os.Getenv(addrEnv)
	}
	if addr == "" {
		logger.Emergency("%s env should not be null", addrEnv)
	}
	if portEnv != "" {
		port := os.Getenv(portEnv)
		if port != "" {
			addr += ":" + port
		}
	}

	prefix := ""
	if isHttps {
		prefix = fmt.Sprintf("https://%s", addr)
	} else {
		prefix = fmt.Sprintf("http://%s", addr)
	}

	logger.Info("%s = %s", name, prefix)

	return prefix
}

func InitGateWay() {
	var durPhase time.Duration
	phaseStep := time.Hour / NumDfRegions

	osAdminClients = make(map[string]*openshift.OpenshiftClient, NumDfRegions)

	osAdminClients[DfRegion_CnNorth01] = BuildDataFoundryClient("DATAFOUNDRY_INFO_CN_NORTH_1", durPhase)
	durPhase += phaseStep
	osAdminClients[DfRegion_CnNorth02] = BuildDataFoundryClient("DATAFOUNDRY_INFO_CN_NORTH_2", durPhase)
	durPhase += phaseStep

	RechargeSercice = BuildServiceUrlPrefixFromEnv("ChargeSercice", false, os.Getenv("ENV_NAME_DATAFOUNDRYRECHARGE_SERVICE_HOST"), os.Getenv("ENV_NAME_DATAFOUNDRYRECHARGE_SERVICE_PORT"))
}

//=============================================================
//get username
//=============================================================

func getDFUserame(token, region string) (string, error) {
	//Logger.Info("token = ", token)
	//if Debug {
	//	return "liuxu", nil
	//}

	user, err := authDF(token, region)
	if err != nil {
		return "", err
	}
	return dfUser(user), nil
}

func authDF(userToken, region string) (*userapi.User, error) {
	if Debug {
		return &userapi.User{
			ObjectMeta: kapi.ObjectMeta{
				Name: "local",
			},
		}, nil
	}

	u := &userapi.User{}
	//osRest := openshift.NewOpenshiftREST(openshift.NewOpenshiftClient(userToken))
	oc := osAdminClients[region]
	if oc == nil {
		return nil, fmt.Errorf("user noud found @ region (%s).")
	}
	oc = oc.NewOpenshiftClient(userToken)
	osRest := openshift.NewOpenshiftREST(oc)

	uri := "/users/~"
	osRest.OGet(uri, u)
	if osRest.Err != nil {
		logger.Info("authDF, region(%s), uri(%s) error: %s", region, uri, osRest.Err)
		//Logger.Infof("authDF, region(%s), token(%s), uri(%s) error: %s", region, userToken, uri, osRest.Err)
		return nil, osRest.Err
	}

	return u, nil
}

func dfUser(user *userapi.User) string {
	return user.Name
}

//====================================================
//call recharge api
//====================================================

type UnifyOrderReq struct {
	Appid            string `xml:"appid"`
	Mch_id           string `xml:"mch_id"`
	Nonce_str        string `xml:"nonce_str"`
	Sign             string `xml:"sign"`
	Body             string `xml:"body"`
	Out_trade_no     string `xml:"out_trade_no"`
	Total_fee        int32  `xml:"total_fee"`
	Spbill_create_ip string `xml:"spbill_create_ip"`
	Notify_url       string `xml:"notify_url"`
	Trade_type       string `xml:"trade_type"`
}

type UnifyOrderResp struct {
	Return_code  string `xml:"return_code"`
	Return_msg   string `xml:"return_msg"`
	Appid        string `xml:"appid"`
	Mch_id       string `xml:"mch_id"`
	Nonce_str    string `xml:"nonce_str"`
	Sign         string `xml:"sign"`
	Result_code  string `xml:"result_code"`
	Prepay_id    string `xml:"prepay_id"`
	Trade_type   string `xml:"trade_type"`
	Code_url     string `xml:"code_url"`
	Err_code     string `xml:"err_code"`
	Err_code_des string `xml:"err_code_des"`
}

//调用微信API统一下单
func unifiedOrders(amount float32) (*models.OrderResult, error) {
	logger.Info("Begin start unified orders.")

	myReq := UnifyOrderReq{
		Appid:            appid,
		Mch_id:           mch_id,
		Body:             "铸数工坊充值",
		Spbill_create_ip: "192.168.12.71",
		Notify_url:       notify_url,
		Trade_type:       "NATIVE",
	}

	myReq.Nonce_str = genNonce_str()
	myReq.Out_trade_no = genOut_trade_no()
	myReq.Total_fee = (int32)(amount * 100)

	var m map[string]interface{}
	m = make(map[string]interface{}, 0)
	m["appid"] = myReq.Appid
	m["body"] = myReq.Body
	m["mch_id"] = myReq.Mch_id
	m["notify_url"] = myReq.Notify_url
	m["trade_type"] = myReq.Trade_type
	m["spbill_create_ip"] = myReq.Spbill_create_ip
	m["total_fee"] = myReq.Total_fee
	m["out_trade_no"] = myReq.Out_trade_no
	m["nonce_str"] = myReq.Nonce_str
	myReq.Sign = wxpayCalcSign(m, wechat_key) //这个是计算wxpay签名的函数上面已贴出
	logger.Info("order sign: %v", myReq.Sign)

	inputBody, err := xml.Marshal(myReq)
	if err != nil {
		logger.Error("Marshal err: %v", err)
		return nil, err
	}

	logger.Debug("input: %v", string(inputBody))

	//t1 := time.Now()
	url := "https://api.mch.weixin.qq.com/pay/unifiedorder"
	//resp, data, err := common.RemoteCallWithBody("POST", url, "", "", inputBody, "text/html")
	//if err != nil {
	//	logger.Error("RemoteCallWithBody err: %v", err)
	//	return nil, err
	//}

	resp := requestTime(url, inputBody)
	//fmt.Println(resp)
	defer resp.Body.Close()

	//d := time.Since(t1)
	//logger.Info("duration: %v", d)

	data, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println("data: %v", data)


	if resp.StatusCode != http.StatusOK {
		logger.Error("unknow err: %v", string(data))
		return nil, errors.New("call weixin api err")
	}

	logger.Info("return data: %v", string(data))

	reqResult := &UnifyOrderResp{}
	err = xml.Unmarshal(data, reqResult)
	if err != nil {
		logger.Error("Unmarshal err: %v", err)
		return nil, err
	}

	if reqResult.Result_code == "FAIL" {
		logger.Warn("微信支付统一下单失败，原因是: %v", reqResult.Return_msg)
		return nil, errors.New(reqResult.Return_msg)
	}

	return &models.OrderResult{
		myReq.Out_trade_no,
		myReq.Nonce_str,
		reqResult.Trade_type,
		amount,
		reqResult.Prepay_id,
		reqResult.Code_url,
		myReq.Sign,
	}, nil

	return nil, nil
}

//微信支付计算签名的函数
func wxpayCalcSign(mReq map[string]interface{}, key string) (sign string) {
	fmt.Println("微信支付签名计算, API KEY:", key)
	//STEP 1, 对key进行升序排序.
	sorted_keys := make([]string, 0)
	for k, _ := range mReq {
		sorted_keys = append(sorted_keys, k)
	}

	sort.Strings(sorted_keys)

	//STEP2, 对key=value的键值对用&连接起来，略过空值
	var signStrings string
	for _, k := range sorted_keys {
		logger.Info("k=%v, v=%v\n", k, mReq[k])
		value := fmt.Sprintf("%v", mReq[k])
		if value != "" {
			signStrings = signStrings + k + "=" + value + "&"
		}
	}

	//STEP3, 在键值对的最后加上key=API_KEY
	if key != "" {
		signStrings = signStrings + "key=" + key
	}

	//STEP4, 进行MD5签名并且将所有字符转为大写.
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(signStrings))
	cipherStr := md5Ctx.Sum(nil)
	upperSign := strings.ToUpper(hex.EncodeToString(cipherStr))
	return upperSign
}

func genNonce_str() string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = randNumber[rand.Intn(len(randNumber))]
	}
	return string(b)
}

func genRandomNumber(amount int) string {
	b := make([]byte, amount)
	for i := range b {
		b[i] = randNumber[rand.Intn(len(randNumber))]
	}
	return string(b)
}

func genOut_trade_no() string {
	t := time.Now().Format("20060102")
	s := genRandomNumber(24)
	return t + s
}

func dfRecharge(region, reason, username, namespace string, amount float32) error {
	logger.Info("Call remote recharge....")
	body := fmt.Sprintf(
		`{"namespace":"%s", "amount":%.3f, "reason":"%s", "user":"%s", "paymode":"%s"}`,
		namespace, amount, reason, username, "wechat",
	)

	//RechargeSercice1 := "http://datafoundry.recharge.app.dataos.io:80"
	url := fmt.Sprintf("%s/charge/v1/couponrecharge?region=%s", RechargeSercice, region)

	oc := osAdminClients[region]
	logger.Info("Call %s recharge. token: %s", url, oc.BearerToken())
	response, data, err := common.RemoteCallWithJsonBody("POST", url, oc.BearerToken(), "", []byte(body))
	if err != nil {
		logger.Error("recharge err: %v", err)
		return err
	}

	if response.StatusCode != http.StatusOK {
		logger.Info("makeRecharge remote (%s) status code: %d. data=%s", url, response.StatusCode, string(data))
		return fmt.Errorf("makeRecharge remote (%s) status code: %d.", url, response.StatusCode)
	}

	return nil
}

func requestTime(url string, body []byte) *http.Response {
	//var show bool
	//
	//flag.BoolVar(&show, "show", false, "Display the response content")
	//flag.Parse()

	//url := flag.Args()[0]
	fmt.Println("URL:", url)

	tp := newTransport()
	client := &http.Client{Transport: tp}

	b := bytes.NewReader(body)

	resp, err := client.Post(url, "text/html", b)
	if err != nil {
		log.Fatalf("get error: %s: %s", err, url)
	}
	//defer resp.Body.Close()

	//show := true
	//output := ioutil.Discard
	//if show {
	//	output = os.Stdout
	//}
	//io.Copy(output, resp.Body)

	log.Println("Duration:", tp.Duration())
	log.Println("Request duration:", tp.ReqDuration())
	log.Println("Connection duration:", tp.ConnDuration())

	return resp
}

type customTransport struct {
	rtp       http.RoundTripper
	dialer    *net.Dialer
	connStart time.Time
	connEnd   time.Time
	reqStart  time.Time
	reqEnd    time.Time
}

func newTransport() *customTransport {

	tr := &customTransport{
		dialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
	tr.rtp = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                tr.dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return tr
}

func (tr *customTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.reqStart = time.Now()
	resp, err := tr.rtp.RoundTrip(r)
	tr.reqEnd = time.Now()
	return resp, err
}

func (tr *customTransport) dial(network, addr string) (net.Conn, error) {
	tr.connStart = time.Now()
	cn, err := tr.dialer.Dial(network, addr)
	tr.connEnd = time.Now()
	return cn, err
}

func (tr *customTransport) ReqDuration() time.Duration {
	return tr.Duration() - tr.ConnDuration()
}

func (tr *customTransport) ConnDuration() time.Duration {
	return tr.connEnd.Sub(tr.connStart)
}

func (tr *customTransport) Duration() time.Duration {
	return tr.reqEnd.Sub(tr.reqStart)
}