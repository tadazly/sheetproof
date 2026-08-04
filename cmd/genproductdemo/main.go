package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

type sheetData struct {
	name string
	rows [][]any
}

func main() {
	dir := flag.String("dir", filepath.Join("build", "showcase", "AetherFront", "configs"), "output directory")
	flag.Parse()

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fatal(err)
	}

	leftPath := filepath.Join(*dir, "balance_worktree.xlsx")
	rightPath := filepath.Join(*dir, "balance_origin-main.xlsx")
	positionLeftPath := filepath.Join(*dir, "balance_position-left.xlsx")
	positionRightPath := filepath.Join(*dir, "balance_position-right.xlsx")
	mergedPath := filepath.Join(*dir, "balance_merged-left.xlsx")
	if err := writeWorkbook(leftPath, leftSheets()); err != nil {
		fatal(err)
	}
	if err := writeWorkbook(rightPath, rightSheets()); err != nil {
		fatal(err)
	}
	positionLeft := leftSheets()
	positionRight := rightSheets()
	positionLeft[0].rows[0][0] = "序号"
	positionRight[0].rows[0][0] = "序号"
	if err := writeWorkbook(positionLeftPath, positionLeft); err != nil {
		fatal(err)
	}
	if err := writeWorkbook(positionRightPath, positionRight); err != nil {
		fatal(err)
	}
	merged := leftSheets()
	merged[0].rows[1][4] = 1360
	if err := writeWorkbook(mergedPath, merged); err != nil {
		fatal(err)
	}

	fmt.Printf("generated product demo:\n  %s\n  %s\n  %s\n  %s\n  %s\n", leftPath, rightPath, positionLeftPath, positionRightPath, mergedPath)
}

func leftSheets() []sheetData {
	return []sheetData{
		{name: "角色成长", rows: [][]any{
			{"id", "role_key", "名称", "职业", "生命", "攻击", "防御", "暴击率", "移速", "解锁等级", "资源路径"},
			{1001, "guardian_lorne", "铁卫·洛恩", "坦克", 1280, 72, 118, 0.05, 4.2, 1, "Role/Guardian/Lorne"},
			{1002, "ranger_ayla", "苍羽·艾拉", "游侠", 860, 142, 54, 0.18, 5.1, 1, "Role/Ranger/Ayla"},
			{1003, "mage_noah", "星术师·诺亚", "法师", 790, 158, 46, 0.12, 4.6, 5, "Role/Mage/Noah"},
			{1004, "healer_mia", "圣愈·米娅", "辅助", 920, 96, 70, 0.08, 4.8, 8, "Role/Healer/Mia"},
			{1005, "fighter_karn", "赤刃·卡恩", "战士", 1080, 132, 82, 0.15, 4.7, 10, "Role/Fighter/Karn"},
			{1006, "frost_eve", "霜语·伊芙", "法师", 810, 151, 49, 0.13, 4.5, 12, "Role/Mage/Eve"},
			{1007, "warden_grom", "岩心·格罗姆", "坦克", 1450, 65, 134, 0.04, 3.9, 15, "Role/Guardian/Grom"},
			{1008, "assassin_sia", "夜巡·希娅", "刺客", 760, 176, 44, 0.24, 5.4, 18, "Role/Assassin/Sia"},
			{1009, "gunner_vick", "雷铳·维克", "射手", 850, 164, 52, 0.20, 5.0, 20, "Role/Gunner/Vick"},
			{1010, "druid_frey", "森灵·芙蕾", "辅助", 900, 102, 66, 0.10, 4.9, 22, "Role/Healer/Frey"},
		}},
		{name: "关卡掉落", rows: [][]any{
			{"id", "stage_key", "关卡名称", "体力", "金币", "经验", "首通道具", "首通数量", "常规掉落", "权重"},
			{2101, "forest_01", "雾林外围", 6, 420, 120, "gem", 30, "wood_shard", 100},
			{2102, "forest_02", "古树回廊", 6, 460, 132, "stamina", 20, "wood_shard", 100},
			{2103, "forest_boss", "荆棘守望者", 10, 880, 260, "hero_token", 5, "guardian_core", 35},
			{2201, "ruins_01", "沉没遗迹", 8, 560, 168, "gem", 35, "aether_dust", 100},
			{2202, "ruins_02", "回声穹顶", 8, 610, 182, "stamina", 25, "aether_dust", 100},
			{2203, "ruins_boss", "无声构装体", 12, 1120, 330, "hero_token", 6, "machine_core", 30},
			{2301, "snow_01", "霜原前哨", 10, 720, 220, "gem", 40, "frost_crystal", 100},
			{2302, "snow_02", "白夜峡谷", 10, 780, 238, "stamina", 30, "frost_crystal", 100},
			{2303, "snow_boss", "凛冬巨像", 14, 1380, 410, "hero_token", 8, "giant_heart", 25},
		}},
		{name: "技能参数", rows: [][]any{
			{"id", "skill_key", "技能名称", "伤害系数", "冷却秒", "能量消耗", "目标规则", "效果资源"},
			{3101, "shield_bash", "壁垒冲击", 1.35, 8, 20, "nearest_enemy", "Fx/Skill/ShieldBash"},
			{3102, "wind_arrow", "穿云箭", 1.82, 6, 18, "lowest_hp_enemy", "Fx/Skill/WindArrow"},
			{3103, "star_fall", "星陨", 2.45, 14, 45, "enemy_cluster", "Fx/Skill/StarFall"},
			{3104, "holy_tide", "圣愈潮汐", 0.95, 12, 38, "lowest_hp_ally", "Fx/Skill/HolyTide"},
			{3105, "red_arc", "赤刃连斩", 2.10, 10, 32, "front_row", "Fx/Skill/RedArc"},
			{3106, "frost_ring", "霜环", 1.58, 11, 34, "enemy_cluster", "Fx/Skill/FrostRing"},
		}},
	}
}

func rightSheets() []sheetData {
	sheets := leftSheets()

	// 赛季平衡版本：角色展示名、坦克生命、游侠攻击、法师解锁等级与辅助移速发生调整。
	sheets[0].rows[1][2] = "铁壁·洛恩"
	sheets[0].rows[1][4] = 1360
	sheets[0].rows[2][5] = 150
	sheets[0].rows[3][9] = 3
	sheets[0].rows[4][8] = 5.0
	// 新角色插入现有记录中间，用于展示主键对齐与按行号比较的区别。
	newRole := []any{1011, "tide_ilan", "潮汐·伊澜", "辅助", 940, 108, 68, 0.09, 4.8, 24, "Role/Healer/Ilan"}
	insertAt := 4
	sheets[0].rows = append(sheets[0].rows, nil)
	copy(sheets[0].rows[insertAt+1:], sheets[0].rows[insertAt:])
	sheets[0].rows[insertAt] = newRole

	// 主分支尚未包含雪原 Boss，并调整了遗迹关卡的产出。
	sheets[1].rows[4][4] = 600
	sheets[1].rows[6][9] = 35
	sheets[1].rows = sheets[1].rows[:len(sheets[1].rows)-1]

	// 技能平衡包含系数、冷却与能量消耗变化。
	sheets[2].rows[2][3] = 1.92
	sheets[2].rows[3][4] = 13
	sheets[2].rows[5][5] = 30

	return sheets
}

func writeWorkbook(path string, sheets []sheetData) error {
	f := excelize.NewFile()
	defer f.Close()

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"17324C"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	percentStyle, err := f.NewStyle(&excelize.Style{NumFmt: 10})
	if err != nil {
		return err
	}

	for index, sheet := range sheets {
		if index == 0 {
			if err := f.SetSheetName("Sheet1", sheet.name); err != nil {
				return err
			}
		} else if _, err := f.NewSheet(sheet.name); err != nil {
			return err
		}

		for rowIndex, row := range sheet.rows {
			for columnIndex, value := range row {
				cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
				if err != nil {
					return err
				}
				if err := f.SetCellValue(sheet.name, cell, value); err != nil {
					return err
				}
			}
		}

		lastColumn, err := excelize.ColumnNumberToName(len(sheet.rows[0]))
		if err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet.name, "A1", lastColumn+"1", headerStyle); err != nil {
			return err
		}
		if sheet.name == "角色成长" {
			if err := f.SetCellStyle(sheet.name, "H2", fmt.Sprintf("H%d", len(sheet.rows)), percentStyle); err != nil {
				return err
			}
		}
		if err := f.SetRowHeight(sheet.name, 1, 24); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "A", "A", 10); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "B", "B", 20); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "C", "D", 15); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "E", lastColumn, 13); err != nil {
			return err
		}
		if err := f.SetPanes(sheet.name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return err
		}
		if err := f.AutoFilter(sheet.name, "A1:"+lastColumn+"1", nil); err != nil {
			return err
		}
	}

	return f.SaveAs(path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
