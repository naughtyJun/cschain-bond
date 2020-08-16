package service

import (
	"cschain-bond/api"
	"cschain-bond/entity"
	"cschain-bond/gorm"
	"cschain-bond/types"
	"cschain-bond/utils"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Start() {
	bz := utils.GetFromUrl("http://www.szse.cn/api/report/ShowReport/data?SHOWTYPE=JSON&CATALOGID=scsj_zqydgk&TABKEY=tab1&txtQueryDate=2020-08")
	bonds := utils.BzToBonds(bz)

	var tokenData types.TokenData
	utils.ParseToResult(bonds, &tokenData)
	tokenDataBz, err := json.Marshal(tokenData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unmarshalJson: %v\n", err)
		os.Exit(1)
	}

	// use sdk to IssueDenom, MintNFT
	denomID := strings.ToLower(utils.RandStringOfLength(4))
	denomName := strings.ToLower(utils.RandStringOfLength(4))
	schema := utils.GetScheme()
	if err = issueDenom(denomID, denomName, schema); err != nil {
		// TODO handle the error
		panic(err)
	}

	tokenID := strings.ToLower(utils.RandStringOfLength(7))
	tokenName := strings.ToLower(utils.RandStringOfLength(7))
	if err = mintNFT(denomID, tokenID, tokenName, string(tokenDataBz)); err != nil {
		// TODO handle the error
		panic(err)
	}
}

func Second() {
	// TODO query collection by restApi
	nftData := api.QueryNfts("yoeu")

	nameIdMap := slice2Map()
	txs := make([]entity.BondTransaction, 0)

	// construct ever row of data, then push ever data in slice
	denomId := nftData.DenomId
	nftId := nftData.NftId
	owner := nftData.Owner
	tokenUri := nftData.TokenUri

	var tokenData types.TokenData
	if err := json.Unmarshal([]byte(nftData.TokenDataStr), &tokenData); err != nil {
		// TODO handle the error
		panic(err)
	}

	market := tokenData.Report.FixedValueHeader.Value
	date := tokenData.Report.Date
	for _, data := range tokenData.Report.Data {
		var amount float64
		if len(data[0]) > 0 {
			var err error
			amount, err = strconv.ParseFloat(data[0], 64)
			if err != nil {
				// TODO hanle the error
				panic(err)
			}
		}

		// get bondCategoryName or repurchaseCategory from data ex[]
		var bondCategoryName, repurchaseCategory string
		if len(data[1]) != 0 {
			bondCategoryName = data[1]
		}

		if len(data[2]) != 0 {
			repurchaseCategory = data[2]
		}

		tx := entity.BondTransaction{
			NftId: nftId,
			// TODO SourceType ,Visible when demand is determined
			SourceType:         0,
			DenomId:            denomId,
			Owner:              owner,
			Uri:                tokenUri,
			Visible:            true,
			Amount:             amount,
			Market:             market,
			StartDate:          utils.String2Time(date.StartDate),
			EndDate:            utils.String2Time(date.EndDate),
			PeriodCategory:     date.Period,
			BondCategory:       nameIdMap[bondCategoryName],
			RepurchaseCategory: nameIdMap[repurchaseCategory],
		}
		txs = append(txs, tx)
	}
	// TODO 批量写入
	BatchInsert(txs)

}

// ex[{1 国债 1 3} {2 地方政府债 0 2001} {3 政策性金融债 2 5}] -> [国债:1,地方政府债:2,政策性金融债:3]
func slice2Map() map[string]int {
	var res = make(map[string]int)

	var b gorm.BondVariety
	bs := b.FindAll()
	for _, v := range bs {
		res[v.Name] = v.ID
	}

	var r gorm.RepurchaseVariety
	rs := r.FindAll()
	for _, v := range rs {
		res[v.Name] = v.ID
	}
	return res
}

func BatchInsert(bondTxs []entity.BondTransaction) {
	db := utils.GetConnection()
	defer db.Close()

	tx := db.Begin()
	sql := `INSERT INTO bond_transaction(
					nft_id,source_type,denom_id,owner,uri,
					visible,amount,market,start_date,
					end_date,period_category,bond_category,repurchase_category) 
			VALUES`
	var vals []interface{}
	const rowSql = "(?,?,?,?,?,?,?,?,?,?,?,?,?)"
	var inserts []string

	for _, bondTx := range bondTxs {
		inserts = append(inserts, rowSql)
		vals = append(vals,
			bondTx.NftId, bondTx.SourceType, bondTx.DenomId, bondTx.Owner, bondTx.Uri,
			bondTx.Visible, bondTx.Amount, bondTx.Market, bondTx.StartDate,
			bondTx.EndDate, bondTx.PeriodCategory, bondTx.BondCategory, bondTx.RepurchaseCategory)
	}

	sql = sql + strings.Join(inserts, ",")
	err := tx.Exec(sql, vals...).Error
	if err != nil {
		tx.Rollback()
		fmt.Println(err)
	}
	tx.Commit()
}