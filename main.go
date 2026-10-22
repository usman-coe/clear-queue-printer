package main

import (
	"fmt"

	"clearqueue/spooler"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

func main() {
	var (
		mw                *walk.MainWindow
		list              *walk.ListBox
		status            *walk.TextLabel
		clearBtn, testBtn *walk.PushButton
		printers          []spooler.Printer
	)

	refresh := func() {
		ps, err := spooler.List()
		if err != nil {
			status.SetText("อ่านรายชื่อเครื่องพิมพ์ไม่ได้: " + err.Error())
		}
		printers = ps
		items, sel := make([]string, len(ps)), 0
		for i, p := range ps {
			jobs := "ไม่มีงานค้าง"
			if p.Jobs > 0 {
				jobs = fmt.Sprintf("ค้าง %d งาน", p.Jobs)
			}
			items[i] = fmt.Sprintf("%s   (%s)", p.Name, jobs)
			if p.Default {
				items[i] += "   ★ เครื่องหลัก"
				sel = i
			}
		}
		list.SetModel(items)
		if len(items) > 0 {
			list.SetCurrentIndex(sel)
		}
	}

	setBusy := func(busy bool, msg string) {
		clearBtn.SetEnabled(!busy)
		testBtn.SetEnabled(!busy)
		status.SetText(msg)
	}

	onClear := func() {
		setBusy(true, "กำลังเคลียร์คิว... กรุณารอสักครู่")
		go func() {
			err := spooler.Clear()
			mw.Synchronize(func() {
				refresh()
				if err != nil {
					setBusy(false, "เคลียร์ไม่สำเร็จ: "+err.Error())
					return
				}
				setBusy(false, "✔ เรียบร้อย! สั่งพิมพ์ใหม่ได้เลย")
			})
		}()
	}

	onTest := func() {
		i := list.CurrentIndex()
		if i < 0 || i >= len(printers) {
			status.SetText("กรุณาเลือกเครื่องพิมพ์ก่อน")
			return
		}
		if err := spooler.PrintTestPage(printers[i].Name); err != nil {
			status.SetText("พิมพ์หน้าทดสอบไม่ได้: " + err.Error())
			return
		}
		status.SetText("ส่งหน้าทดสอบไปที่ " + printers[i].Name + " แล้ว")
	}

	err := MainWindow{
		AssignTo: &mw,
		Title:    "เคลียร์คิวเครื่องพิมพ์",
		Icon:     1, // go-winres stores the app icon as resource ID 1; walk only looks for ID 7 on its own
		Size:     Size{Width: 520, Height: 500},
		Font:     Font{Family: "Leelawadee UI", PointSize: 11},
		Layout:   VBox{Margins: Margins{Left: 16, Top: 16, Right: 16, Bottom: 16}, Spacing: 10},
		Children: []Widget{
			Label{Text: "เครื่องพิมพ์ในคอมพิวเตอร์เครื่องนี้:"},
			ListBox{AssignTo: &list, StretchFactor: 1},
			PushButton{
				AssignTo:  &clearBtn,
				Text:      "เคลียร์คิวงานพิมพ์ที่ค้าง",
				MinSize:   Size{Height: 70},
				Font:      Font{Family: "Leelawadee UI", PointSize: 16, Bold: true},
				OnClicked: onClear,
			},
			PushButton{AssignTo: &testBtn, Text: "พิมพ์หน้าทดสอบ", MinSize: Size{Height: 40}, OnClicked: onTest},
			TextLabel{AssignTo: &status, Text: "พร้อมใช้งาน", MinSize: Size{Width: 1}},
			TextLabel{
				Text:          "พัฒนาโดย นายอุสมาน การีมี\nนักวิชาการคอมพิวเตอร์ชำนาญการ สสอ.หาดใหญ่",
				TextAlignment: AlignHCenterVNear,
				TextColor:     walk.RGB(120, 120, 120),
				Font:          Font{Family: "Leelawadee UI", PointSize: 9},
			},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "เคลียร์คิวเครื่องพิมพ์", err.Error(), walk.MsgBoxIconError)
		return
	}
	refresh()
	mw.Run()
}
