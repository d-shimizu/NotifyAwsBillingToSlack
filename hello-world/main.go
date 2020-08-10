package main

import (
	"fmt"
	"time"

	"log"

	"encoding/json"
	"net/http"
	"bytes"

	"sort"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/costexplorer"
    "github.com/aws/aws-sdk-go/service/ssm"

    "github.com/aws/aws-sdk-go/service/sts"
    "github.com/aws/aws-sdk-go/aws/awserr"
//	"github.com/aws/aws-sdk-go/aws/credentials"
)

// *CostInfoコンストラクタで戻り値の型を定義する
type TotalBillingInfo struct{
	Start string `json:"start"`
	End string `json:"end"`
	Totalbilling string `json:"totalbilling"`
}

type ServiceBillingInfo struct {
	Service string `json:"service"`
	Billing string `json:"billing"`
}

type SlackMessage struct {
	Text string `json:"text"`
	Color string `json:"color"`
	Username string `json:"username"`
	Icon_emoji string `json:"icon_emoji"`
}

/*
type SlackPayload struct {
	Username string `json:"username"`
	Icon string `json:"icon"`
	Attachments SlackMessage `json:"attachments"`
}
*/


// 請求情報を取得する関数
// *TotalBillingInfoコンストラクタを戻り値とする
func NewTotalBilling() *TotalBillingInfo {
	// https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/#NewStaticCredentials
	sess := session.Must(session.NewSession())
	svc := costexplorer.New(
		sess,
		aws.NewConfig().WithRegion("ap-northeast-1"),
	)

	now_date := time.Now()
	end := now_date.Format("2006-01-02")
	end_yesterday := now_date.AddDate(0, 0, -1).Format("2006-01-02")
	start := time.Date(now_date.Year(), now_date.Month(), 1, 0, 0, 0,0, time.UTC).Format("2006-01-02")

	if now_date.Day() == 1 {
		// start = start.AddDate(0, -1, 0)
		start = time.Date(now_date.Year(), now_date.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01-02")
	}

	// https://docs.aws.amazon.com/aws-cost-management/latest/APIReference/API_GetCostAndUsage.html#awscostmanagement-GetCostAndUsage-request-TimePeriod
	// https://gitter.im/aws/aws-sdk-go?at=5df056990995661eb8c4a773
    output, err := svc.GetCostAndUsage(&costexplorer.GetCostAndUsageInput{
        Granularity: aws.String("MONTHLY"),
        Metrics: []*string{
            aws.String("AmortizedCost"),
        },
        TimePeriod: &costexplorer.DateInterval{
            Start: aws.String(start),
            End:   aws.String(end),
        },
    })
    if err != nil {
        panic(err)
    }
	// fmt.Println(output)

    total := output.ResultsByTime[0].Total["AmortizedCost"]
    totalbilling := aws.StringValue(total.Amount)
	// fmt.Println(totalbilling)

    // ポインタで返す
	return &TotalBillingInfo{
        Start:  start,
        End:    end_yesterday,
        Totalbilling: totalbilling,
    }
}

// サービスごとの請求情報を取得する関数
// func NewServiceBilling() map[string]string {
func NewServiceBilling() string {
	sess := session.Must(session.NewSession())
	svc := costexplorer.New(
		sess,
		aws.NewConfig().WithRegion("ap-northeast-1"),
	)


	now_date := time.Now()
	end := now_date.Format("2006-01-02")
	start := time.Date(now_date.Year(), now_date.Month(), 1, 0, 0, 0,0, time.UTC).Format("2006-01-02")

	if now_date.Day() == 1 {
		// start = start.AddDate(0, -1, 0)
		start = time.Date(now_date.Year(), now_date.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01-02")
	}

	// https://docs.aws.amazon.com/aws-cost-management/latest/APIReference/API_GetCostAndUsage.html#awscostmanagement-GetCostAndUsage-request-TimePeriod
	// https://gitter.im/aws/aws-sdk-go?at=5df056990995661eb8c4a773
    output, err := svc.GetCostAndUsage(&costexplorer.GetCostAndUsageInput{
        Granularity: aws.String("MONTHLY"),
        Metrics: []*string{
            aws.String("AmortizedCost"),
        },
        TimePeriod: &costexplorer.DateInterval{
            Start: aws.String(start),
            End:   aws.String(end),
        },
		GroupBy: []*costexplorer.GroupDefinition{{
			Key: aws.String("SERVICE"),
			Type: aws.String("DIMENSION")},
		},
    })
    if err != nil {
        panic(err)
    }

    use_service_count := len(output.ResultsByTime[0].Groups)
	m := make(map[string]string)

	for i := 0 ; i < use_service_count; i++ {
		p := output.ResultsByTime[0].Groups[i].Keys[0]
		// fmt.Println(*p)

		q := output.ResultsByTime[0].Groups[i].Metrics["AmortizedCost"].Amount
		// fmt.Println(*q)

		m[*p] = *q

	}
	// fmt.Println(m)

	tmp := make([]string, use_service_count)
	index := 0
    for key, _ := range m {
        tmp[index] = key
        index++
    }
	sort.Strings(tmp)

	service_billings := ""
	for _, v := range tmp {
		service_billing := v + " :  " + m[v] + "\n"
		service_billings += service_billing
	}
	return service_billings

}

// 構造体が引数にある場合に戻り値の構造体はポインタにできない
func makeSlackMessage(account_id string, totalBillingInfo *TotalBillingInfo, serviceBillingInfo string) SlackMessage {

    return SlackMessage{
	// TotalBillingInfoの型を参照している
        Username: "aws-cost-and-usage-report (webhook)",
        Icon_emoji: ":aws-cost-and-usage-report:",
        Text: fmt.Sprintf("Account ID: %s \n %s ~ %s の請求額は $%s です。\nサービスごとの利用料は以下の通りです。\n ```%s```",
		    account_id,
            totalBillingInfo.Start,
            totalBillingInfo.End,
            totalBillingInfo.Totalbilling,
			serviceBillingInfo,
		),
		Color: "good",
    }

}

func getCallerIdentity() string {
    svc := sts.New(session.New())
	input := &sts.GetCallerIdentityInput{}

    result, err := svc.GetCallerIdentity(input)

	if err != nil {
	    if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            default:
			    fmt.Println(aerr.Error())
			}
		} else {
		    fmt.Println(err.Error())
        }
        // return
	}
	// fmt.Println(*result.Account)
	account_id := *result.Account
    return account_id
}

func PostSlack(message SlackMessage) {
	input, _ := json.Marshal(message)
	fmt.Println(string(input))

    // https://qiita.com/kou_pg_0131/items/1eee0c46f478438aa115
    svc := ssm.New(session.New(), &aws.Config{
	        Region: aws.String("ap-northeast-1"),
	})

    res, err := svc.GetParameter(&ssm.GetParameterInput{
        Name:           aws.String("NotifyAwsBillingToSlack.WebHookUrl"),
        WithDecryption: aws.Bool(true),
    })
    if err != nil {
       // エラー時の処理
       log.Fatal(err)
    }
	webhook_url := *res.Parameter.Value

	// http.Post(SlackApi, "application/json", bytes.NewBuffer(input))
	http.Post(webhook_url, "application/json", bytes.NewBuffer(input))
}


func BillingNotification() {
	totalBillingInfo := NewTotalBilling()
	serviceBillingInfo := NewServiceBilling()
    account_id := getCallerIdentity()

	message := makeSlackMessage(account_id, totalBillingInfo, serviceBillingInfo)

	PostSlack(message)
}

func main() {
    lambda.Start(BillingNotification)
}
