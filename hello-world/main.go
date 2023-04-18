package main

import (
	"fmt"
	"time"

	"log"

	"bytes"
	"encoding/json"
	"net/http"

	"sort"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sts"
)

// トータル請求情報を格納する構造体
type TotalBillingInfo struct {
	startDate    string `json:"start_date"`
	endDate      string `json:"end_date"`
	totalBilling string `json:"total_billing"`
}

// サービスごとの請求情報を格納する構造体
type ServiceBillingInfo struct {
	awsService string `json:"aws_service"`
	billing    string `json:"billing"`
}

// Slackに送信するメッセージを格納する構造体
type SlackMessage struct {
	Text       string `json:"text"`
	Color      string `json:"color"`
	Username   string `json:"username"`
	Icon_emoji string `json:"icon_emoji"`
}

// トータルの請求情報を取得する関数
func GetTotalBillingInfo() *TotalBillingInfo {
	// https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/#NewStaticCredentials
	sess := session.Must(session.NewSession())
	svc := costexplorer.New(
		sess,
		aws.NewConfig().WithRegion("ap-northeast-1"),
	)

	nowDate := time.Now()
	//end := nowDate.Format("2006-01-02")
	endDate := nowDate.AddDate(0, 0, -1).Format("2006-01-02")
	startDate := time.Date(nowDate.Year(), nowDate.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	if nowDate.Day() == 1 {
		startDate = time.Date(nowDate.Year(), nowDate.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01-02")
	}

	// https://docs.aws.amazon.com/aws-cost-management/latest/APIReference/API_GetCostAndUsage.html#awscostmanagement-GetCostAndUsage-request-TimePeriod
	// https://gitter.im/aws/aws-sdk-go?at=5df056990995661eb8c4a773
	output, err := svc.GetCostAndUsage(&costexplorer.GetCostAndUsageInput{
		Granularity: aws.String("MONTHLY"),
		Metrics: []*string{
			aws.String("AmortizedCost"),
		},
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate),
			End:   aws.String(endDate),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	// for debug
	// fmt.Println(output)

	total := output.ResultsByTime[0].Total["AmortizedCost"]
	totalBilling := aws.StringValue(total.Amount)

	return &TotalBillingInfo{
		startDate:    startDate,
		endDate:      endDate,
		totalBilling: totalBilling,
	}
}

// サービスごとの請求情報を取得する関数
func GetServiceBillingInfo() string {
	sess := session.Must(session.NewSession())
	svc := costexplorer.New(
		sess,
		aws.NewConfig().WithRegion("ap-northeast-1"),
	)

	nowDate := time.Now()
	endDate := nowDate.Format("2006-01-02")
	startDate := time.Date(nowDate.Year(), nowDate.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	if nowDate.Day() == 1 {
		startDate = time.Date(nowDate.Year(), nowDate.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01-02")
	}

	// https://docs.aws.amazon.com/aws-cost-management/latest/APIReference/API_GetCostAndUsage.html#awscostmanagement-GetCostAndUsage-request-TimePeriod
	// https://gitter.im/aws/aws-sdk-go?at=5df056990995661eb8c4a773
	costAndUsageResponse, err := svc.GetCostAndUsage(&costexplorer.GetCostAndUsageInput{
		Granularity: aws.String("MONTHLY"),
		Metrics: []*string{
			aws.String("AmortizedCost"),
		},
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate),
			End:   aws.String(endDate),
		},
		GroupBy: []*costexplorer.GroupDefinition{{
			Key:  aws.String("SERVICE"),
			Type: aws.String("DIMENSION")},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	numberOfServiceCounts := len(costAndUsageResponse.ResultsByTime[0].Groups)
	awsServiceAndCostMapping := make(map[string]string)
	// AWS サービスごとの請求情報を取得する
	for i := 0; i < numberOfServiceCounts; i++ {
		awsServiceName := costAndUsageResponse.ResultsByTime[0].Groups[i].Keys[0]
		fmt.Println(*awsServiceName)

		awsServiceCost := costAndUsageResponse.ResultsByTime[0].Groups[i].Metrics["AmortizedCost"].Amount
		fmt.Println(*awsServiceCost)

		awsServiceAndCostMapping[*awsServiceName] = *awsServiceCost

	}
	// for debug
	fmt.Println(awsServiceAndCostMapping)

	awsServices := make([]string, numberOfServiceCounts)
	index := 0
	for key, _ := range awsServiceAndCostMapping {
		awsServices[index] = key
		index++
	}
	sort.Strings(awsServices)

	//service_billings := ""
	var awsServiceBillings string
	for _, v := range awsServices {
		awsServiceBilling := v + " :  " + awsServiceAndCostMapping[v] + "\n"
		awsServiceBillings += awsServiceBilling
	}
	return awsServiceBillings

}

// 構造体が引数にある場合に戻り値の構造体はポインタにできない
// Slack へのメッセージを作成する関数
func makeSlackMessage(awsAccountID string, totalBillingInfo *TotalBillingInfo, serviceBillingInfo string) SlackMessage {

	return SlackMessage{
		// TotalBillingInfoの型を参照している
		Username:   "aws-cost-and-usage-report (webhook)",
		Icon_emoji: ":aws-cost-and-usage-report:",
		Text: fmt.Sprintf("Account ID: %s \n %s ~ %s の請求額は $%s です。\nサービスごとの利用料は以下の通りです。\n ```%s```",
			awsAccountID,
			totalBillingInfo.startDate,
			totalBillingInfo.endDate,
			totalBillingInfo.totalBilling,
			serviceBillingInfo,
		),
		Color: "good",
	}

}

func GetAwsAccountID() string {
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
	}
	awsAccountID := *result.Account
	return awsAccountID
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
		log.Fatal(err)
	}
	slackWebhookURL := *res.Parameter.Value

	http.Post(slackWebhookURL, "application/json", bytes.NewBuffer(input))
}

// AWS 請求情報を取得してSlackに通知する関数
func awsBillingNotification() {
	totalBillingInfo := GetTotalBillingInfo()
	//fmt.Println(totalBillingInfo)
	serviceBillingInfo := GetServiceBillingInfo()
	//fmt.Println(serviceBillingInfo)
	awsAccountID := GetAwsAccountID()

	message := makeSlackMessage(awsAccountID, totalBillingInfo, serviceBillingInfo)

	PostSlack(message)

}

func main() {
	lambda.Start(awsBillingNotification)
}
