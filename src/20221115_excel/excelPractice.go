package main

import (
	"github.com/xuri/excelize/v2"
	"log"
)

func main() {
	f := excelize.NewFile()

	/*
			f.SetCellValue("Sheet1", "A1", "Sunny Day")
			f.MergeCell("Sheet1", "A1", "B2")

			style, _ := f.NewStyle(`{"alignment":{"horizontal":"center","vertical":"center"},
		        "font":{"bold":true,"italic":true}}`)
			f.SetCellStyle("Sheet1", "A1", "B2", style)
	*/

	f.SetCellValue("Sheet1", "A1", "일자")
	f.SetColWidth("Sheet1", "A", "A", 15)
	f.MergeCell("Sheet1", "A1", "A2")

	f.SetCellValue("Sheet1", "B1", "통화수")
	f.SetColWidth("Sheet1", "B", "B", 10)
	f.MergeCell("Sheet1", "B1", "B2")

	f.SetCellValue("Sheet1", "C1", "음성ARS비율")
	f.SetColWidth("Sheet1", "C", "C", 15)
	f.MergeCell("Sheet1", "C1", "C2")

	f.SetCellValue("Sheet1", "D1", "SMS")
	f.SetColWidth("Sheet1", "D", "F", 5)
	f.MergeCell("Sheet1", "D1", "F1")

	f.SetCellValue("Sheet1", "D2", "요청")
	f.SetCellValue("Sheet1", "E2", "접속")
	f.SetCellValue("Sheet1", "F2", "%")

	if err := f.SaveAs("simple.xlsx"); err != nil {
		log.Fatal(err)
	}

}
