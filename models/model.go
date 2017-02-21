package models

import (
	"database/sql"
)

type OrderResult struct {
	Out_trade_no string
	Nonce_str    string
	Trade_type   string
	Total_fee    float32
	Prepay_id    string
	Code_url     string
	Sign         string
}

func CreateOrder(db *sql.DB, result *OrderResult, region, username, namespace string) error {
	logger.Info("Begin create a order.")

	sql := "insert into DF_WECHATORDERS (" +
		"OUT_TRADE_NO, NONCE_STR, ORDERSIGN, TOTAL_FEE, TRADE_TYPE, PREPAY_ID, CODE_URL, REGION, USERNAME, NAMESPACE, STATUS) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	_, err := db.Exec(sql, result.Out_trade_no, result.Nonce_str, result.Sign,
		result.Total_fee, result.Trade_type, result.Prepay_id, result.Code_url, region, username, namespace, "created")
	if err != nil {
		logger.Error(" db.Exec err: %v", err)
		return err
	}
	logger.Info("End create a order.")
	return nil
}

type orderInfo struct {
	Out_trade_no string
	Total_fee    float32
	Region       string
	Username     string
	Namespace    string
	Status       string
}

func GetOrderInfo(db *sql.DB, out_trade_no string) (*orderInfo, error) {

	sql := "select OUT_TRADE_NO, TOTAL_FEE, REGION, USERNAME, NAMESPACE, STATUS from DF_WECHATORDERS where OUT_TRADE_NO=?"

	row := db.QueryRow(sql, out_trade_no)

	info := &orderInfo{}
	err := row.Scan(&info.Out_trade_no, &info.Total_fee, &info.Region, &info.Username, &info.Namespace, &info.Status)
	if err != nil {
		logger.Error("row.Scan err: %v", err)
		return nil, err
	}

	return info, nil
}

type WXPayNotifyReq struct {
	Return_code    string `xml:"return_code"`
	Return_msg     string `xml:"return_msg"`
	Appid          string `xml:"appid"`
	Mch_id         string `xml:"mch_id"`
	Nonce          string `xml:"nonce_str"`
	Sign           string `xml:"sign"`
	Result_code    string `xml:"result_code"`
	Openid         string `xml:"openid"`
	Is_subscribe   string `xml:"is_subscribe"`
	Trade_type     string `xml:"trade_type"`
	Bank_type      string `xml:"bank_type"`
	Total_fee      int    `xml:"total_fee"`
	Fee_type       string `xml:"fee_type"`
	Cash_fee       int    `xml:"cash_fee"`
	Cash_fee_Type  string `xml:"cash_fee_type"`
	Transaction_id string `xml:"transaction_id"`
	Out_trade_no   string `xml:"out_trade_no"`
	Attach         string `xml:"attach"`
	Time_end       string `xml:"time_end"`
}

func CompleteOrder(db *sql.DB, reqParams *WXPayNotifyReq) error {
	logger.Info("update order in db.")

	sql := "update DF_WECHATORDERS set CASH_FEE=?, FEE_TYPE=?, BANK_TYPE=?, OPENID=?, TRANSACTION_ID=?, " +
		"TIME_END=?, STATUS=? where OUT_TRADE_NO=?"

	_, err := db.Exec(sql, reqParams.Cash_fee, reqParams.Fee_type, reqParams.Bank_type,
		reqParams.Openid, reqParams.Transaction_id, reqParams.Time_end, "paid", reqParams.Out_trade_no)
	if err != nil {
		logger.Error("update table err: %v", err)
		return err
	}

	logger.Info(">>>>>\n%v\n%v, %v, %v, %v, %v, %v, %v, %v", sql,
		reqParams.Cash_fee, reqParams.Fee_type, reqParams.Bank_type,
		reqParams.Openid, reqParams.Transaction_id, reqParams.Time_end, "paid", reqParams.Out_trade_no)

	return nil
}

func QueryOrder(db *sql.DB, no string) (string, error) {

	sql := "select STATUS from DF_WECHATORDERS where OUT_TRADE_NO=?"

	row := db.QueryRow(sql, no)
	var out_trade_no string
	err := row.Scan(&out_trade_no)
	if err != nil {
		logger.Error("query order err: %v", err)
		return "", err
	}

	return out_trade_no, nil
}
