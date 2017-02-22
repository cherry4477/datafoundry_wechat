package api

import (
	"encoding/xml"
	"fmt"
	"github.com/asiainfoLDP/datafoundry_wechat/common"
	"github.com/asiainfoLDP/datafoundry_wechat/log"
	"github.com/asiainfoLDP/datafoundry_wechat/models"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"os"
	"strings"
)

var ()

const (
	letterBytes = "abcdefghjklmnpqrstuvwxyz0123456789"
	randNumber  = "0123456789"
)

var logger = log.GetLogger()

var (
	AdminUsers = make([]string, 0)
	appid      string
	mch_id     string
	notify_url string
	wechat_key string
)

func init() {
	initAdminUser()
	initWechatParam()
}

type RechargeInfo struct {
	Amount    float32 `json:"amount"`
	Namespace string  `json:"namespace"`
}

func WeChatOrders(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Request url: PUT %v.", r.URL)
	logger.Info("Begin wechat orders handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	r.ParseForm()
	region := r.Form.Get("region")
	//username, e := validateAuth(r.Header.Get("Authorization"), region)
	//if e != nil {
	//	JsonResult(w, http.StatusUnauthorized, e, nil)
	//	return
	//}
	username := "test"
	logger.Debug("username:%v", username)

	correctInput := []string{"amount", "namespace"}
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
		return
	}

	err = models.CreateOrder(db, result, region, username, rechargeInfo.Namespace)
	if err != nil {
		logger.Error("db create order err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeCreateOrder, err.Error()), nil)
		return
	}

	JsonResult(w, http.StatusOK, nil, struct {
		Out_trade_no string  `json:"out_trade_no"`
		Trade_type   string  `json:"trade_type"`
		Total_fee    float32 `json:"total_fee"`
		Code_url     string  `json:"code_url"`
	}{result.Out_trade_no, result.Trade_type, result.Total_fee, result.Code_url})

	logger.Info("End wechat orders handler.")
}

type WXPayNotifyResp struct {
	Return_code string `xml:"return_code"`
	Return_msg  string `xml:"return_msg"`
}

func WeChatCallBack(w http.ResponseWriter, r *http.Request, params httprouter.Params) {

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	reqParams := &models.WXPayNotifyReq{}
	err := common.ParseRequestXmlInto(r, reqParams)
	if err != nil {
		http.Error(w.(http.ResponseWriter), http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	reqMap := make(map[string]interface{}, 0)
	reqMap["return_code"] = reqParams.Return_code
	reqMap["return_msg"] = reqParams.Return_msg
	reqMap["appid"] = reqParams.Appid
	reqMap["mch_id"] = reqParams.Mch_id
	reqMap["nonce_str"] = reqParams.Nonce
	reqMap["result_code"] = reqParams.Result_code
	reqMap["openid"] = reqParams.Openid
	reqMap["is_subscribe"] = reqParams.Is_subscribe
	reqMap["trade_type"] = reqParams.Trade_type
	reqMap["bank_type"] = reqParams.Bank_type
	reqMap["total_fee"] = reqParams.Total_fee
	reqMap["fee_type"] = reqParams.Fee_type
	reqMap["cash_fee"] = reqParams.Cash_fee
	reqMap["cash_fee_type"] = reqParams.Cash_fee_Type
	reqMap["transaction_id"] = reqParams.Transaction_id
	reqMap["out_trade_no"] = reqParams.Out_trade_no
	reqMap["attach"] = reqParams.Attach
	reqMap["time_end"] = reqParams.Time_end

	var resp WXPayNotifyResp
	//进行签名校验
	if wxpayVerifySign(reqMap, reqParams.Sign) {
		resp.Return_code = "SUCCESS"
		resp.Return_msg = "OK"

		orderInfo, err := models.GetOrderInfo(db, reqParams.Out_trade_no)
		if err != nil {
			http.Error(w.(http.ResponseWriter), http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		logger.Debug("get order's info:", orderInfo)

		if orderInfo.Status == "created" {
			dfRechargeFlag := false
			for i := 1; i <= 3; i++ {
				err := dfRecharge(orderInfo.Region, orderInfo.Out_trade_no, orderInfo.Username, orderInfo.Namespace, orderInfo.Total_fee)
				if err != nil {
					logger.Warn("datafoundry充值失败，重试第%d次", i)
					continue
				} else {
					dfRechargeFlag = true
					break
				}
			}

			if !dfRechargeFlag {
				logger.Warn("datafound充值失败，微信退款")
				//todo 微信退款
			}

			err = models.CompleteOrder(db, reqParams)
			if err != nil {
				http.Error(w.(http.ResponseWriter), http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}
	} else {
		resp.Return_code = "FAIL"
		resp.Return_msg = "failed to verify sign, please retry!"
	}

	bytes, _err := xml.Marshal(resp)
	strResp := strings.Replace(string(bytes), "WXPayNotifyResp", "xml", -1)
	if _err != nil {
		fmt.Println("xml编码失败，原因：", _err)
		http.Error(w.(http.ResponseWriter), http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.(http.ResponseWriter).WriteHeader(http.StatusOK)
	fmt.Fprint(w.(http.ResponseWriter), strResp)
}

func wxpayVerifySign(needVerifyM map[string]interface{}, sign string) bool {
	signCalc := wxpayCalcSign(needVerifyM, wechat_key)

	logger.Info("计算出来的sign: %v", signCalc)
	logger.Info("微信异步通知sign: %v", sign)
	if sign == signCalc {
		logger.Info("签名校验通过!")
		return true
	}

	logger.Warn("签名校验失败!")
	return false
}

func QueryOrder(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Info("Begin query order handler.")

	db := models.GetDB()
	if db == nil {
		logger.Warn("Get db is nil.")
		JsonResult(w, http.StatusInternalServerError, GetError(ErrorCodeDbNotInitlized), nil)
		return
	}

	no := params.ByName("no")

	orderStatus, err := models.QueryOrder(db, no)
	if err != nil {
		logger.Error("catch err: %v", err)
		JsonResult(w, http.StatusBadRequest, GetError2(ErrorCodeQueryOrder, err.Error()), nil)
		return
	}

	if orderStatus == "paid" {
		JsonResult(w, http.StatusOK, nil, struct {
			Status   int    `json:"status"`
			Describe string `json:"describe"`
		}{1, "订单已支付"})
	} else {
		JsonResult(w, http.StatusOK, nil, struct {
			Status   int    `json:"status"`
			Describe string `json:"describe"`
		}{0, "订单未支付"})
	}
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

func initWechatParam() {
	appid = os.Getenv("WECHATAPPID")
	if appid == "" {
		logger.Warn("Please set wechat param.")
		os.Exit(1)
	}
	mch_id = os.Getenv("WECHATMCHID")
	if mch_id == "" {
		logger.Warn("Please set wechat param.")
		os.Exit(1)
	}
	notify_url = os.Getenv("WECHATNOTIFYURL")
	if notify_url == "" {
		logger.Warn("Please set wechat param.")
		os.Exit(1)
	}

	wechat_key = os.Getenv("WECHATKEY")
	if notify_url == "" {
		logger.Warn("Please set wechat param.")
		os.Exit(1)
	}
}

func checkAdminUsers(user string) bool {
	for _, adminUser := range AdminUsers {
		if adminUser == user {
			return true
		}
	}

	return false
}
