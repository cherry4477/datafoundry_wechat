package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/asiainfoLDP/datafoundry_wechat/common"
	"github.com/asiainfoLDP/datafoundry_wechat/log"
	"github.com/asiainfoLDP/datafoundry_wechat/models"
	"github.com/julienschmidt/httprouter"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	url string = "https://api.mch.weixin.qq.com/pay/unifiedorder"
)

const (
	letterBytes = "abcdefghjklmnpqrstuvwxyz0123456789"
	randNumber  = "0123456789"
)

var logger = log.GetLogger()

var AdminUsers = make([]string, 0)

func init() {
	initAdminUser()
}

type RechargeInfo struct {
	Amount float32 `json:"amount"`
}

func WeChatPay(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: PUT %v.", r.URL)
	logger.Info("Begin use a coupon handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	r.ParseForm()
	region := r.Form.Get("region")
	username, e := validateAuth(r.Header.Get("Authorization"), region)
	if e != nil {
		JsonResult(w, http.StatusUnauthorized, e, nil)
		return
	}
	logger.Debug("username:%v", username)

	//serial := params.ByName("serial")

	correctInput := []string{"amount"}
	rechargeInfo := &RechargeInfo{}
	err := common.ParseRequestJsonIntoWithValidateParams(r, correctInput, rechargeInfo)
	if err != nil {
		logger.Error("Parse body err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeParseJsonFailed, err.Error()), nil)
		return
	}

	//微信统一下单
	result, err := unifiedOrders(rechargeInfo.Amount)
	if err != nil {
		logger.Error("catch err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeUnkown, err.Error()), nil)
	}

	//couponRecharge(region, serial, username, useInfo.Namespace, rechargeInfo.Amount)

	//getResult, err := models.RetrieveCouponByID(db, useInfo.Code)
	//if err != nil {
	//	logger.Error("db get coupon err: %v", err)
	//	JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeGetCoupon, err.Error()), nil)
	//	return
	//}

	//callback := func() error {
	//	return couponRecharge(region, serial, username, useInfo.Namespace, rechargeInfo.Amount)
	//
	//}

	//result, err := models.UseCoupon(db, useInfo, callback)
	//if err != nil {
	//	JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeUseCoupon, err.Error()), nil)
	//	return
	//}

	//logger.Info("End use a coupon handler.")
	JsonResult(w, http.StatusOK, nil, result)
}

type UnifyOrderReq struct {
	Appid            string  `xml:"appid"`
	Mch_id           string  `xml:"mch_id"`
	Nonce_str        string  `xml:"nonce_str"`
	Sign             string  `xml:"sign"`
	Body             string  `xml:"body"`
	Out_trade_no     string  `xml:"out_trade_no"`
	Total_fee        float32 `xml:"total_fee"`
	Spbill_create_ip string  `xml:"spbill_create_ip"`
	Notify_url       string  `xml:"notify_url"`
	Trade_type       string  `xml:"trade_type"`
	//Openid           string `xml:"openid"`
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

type orderResult struct {
	Out_trade_no string
	Trade_type   string
	Code_url     string
}

func unifiedOrders(amount float32) (*orderResult, error) {
	logger.Info("Begin start unified orders.")

	myReq := UnifyOrderReq{
		Appid:  "wxd653a9d6ef5659ab",
		Mch_id: "1419771302",
		//Nonce_str: "5K8264ILTKCH16CQ2502SI8ZNMTM67VS",
		//Sign:             "C380BEC2BFD727A4B6845133519F3AD6",
		Body: "铸数工坊充值",
		//Out_trade_no:     "201508061125346",
		//Total_fee:        100,
		Spbill_create_ip: "123.12.12.123",
		Notify_url:       "http://www.weixin.qq.com/wxpay/pay.php",
		Trade_type:       "NATIVE",
		//Openid:           "oJtqRwsBv42LOBmpaBxKCs-OVP50",
	}

	myReq.Nonce_str = genNonce_str()
	myReq.Out_trade_no = genOut_trade_no()
	myReq.Total_fee = amount * 100

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
	//m["openid"] = myReq.Openid
	myReq.Sign = wxpayCalcSign(m, "data2016data2016data2016data2016") //这个是计算wxpay签名的函数上面已贴出

	inputBody, err := xml.Marshal(myReq)
	if err != nil {
		logger.Error("Marshal err: %v", err)
		return nil, err
	}

	logger.Debug("input: %v", string(inputBody))

	resp, data, err := common.RemoteCallWithBody("POST", url, "", "", inputBody, "text/html")
	if err != nil {
		logger.Error("RemoteCallWithBody err: %v", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("unknow err: %v", string(data))
		return nil, errors.New("call weixin api err")
	}

	reqResult := &UnifyOrderResp{}
	err = xml.Unmarshal(data, reqResult)
	if err != nil {
		logger.Error("Unmarshal err: %v", err)
		return nil, err
	}

	logger.Info("request return: %v", reqResult)

	if reqResult.Result_code == "FAIL" {
		logger.Warn("微信支付统一下单失败，原因是: %v", reqResult.Return_msg)
		return nil, errors.New(reqResult.Return_msg)
	}

	return &orderResult{
		myReq.Out_trade_no,
		reqResult.Trade_type,
		reqResult.Code_url,
	}, nil

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
		fmt.Printf("k=%v, v=%v\n", k, mReq[k])
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
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func genOut_trade_no() string {
	t := time.Now().Format("20060102")
	str := fmt.Sprintf("%s%d", t, time.Now().Unix())
	return str
}

func validateAuth(token, region string) (string, *Error) {
	if token == "" {
		return "", GetError(ErrorCodeAuthFailed)
	}

	username, err := getDFUserame(token, region)
	if err != nil {
		return "", GetError2(ErrorCodeAuthFailed, err.Error())
	}

	return username, nil
}

func initAdminUser() {
	admins := os.Getenv("ADMINUSERS")
	if admins == "" {
		logger.Warn("Not set admin users.")
	}
	admins = strings.TrimSpace(admins)
	AdminUsers = strings.Split(admins, " ")
	logger.Info("Admin users: %v.", AdminUsers)
}

func checkAdminUsers(user string) bool {
	for _, adminUser := range AdminUsers {
		if adminUser == user {
			return true
		}
	}

	return false
}
