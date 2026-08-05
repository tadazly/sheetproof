package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tadazly/sheetproof/internal/localization"
	"github.com/xuri/excelize/v2"
)

type sheetData struct {
	name string
	rows [][]any
}

type demoText struct {
	sheets          [3]string
	positionHeader  string
	characterHeader []any
	stageHeader     []any
	skillHeader     []any
	characters      [11]string
	roles           [6]string
	stages          [9]string
	skills          [6]string
}

func main() {
	dir := flag.String("dir", filepath.Join("build", "showcase", "AetherFront", "configs"), "base output directory")
	localeFlag := flag.String("locale", "en", "demo locale: en, zh-CN, or ja")
	allLocales := flag.Bool("all-locales", false, "generate en, zh-CN, and ja demos")
	flag.Parse()

	locales := []localization.Locale{}
	if *allLocales {
		locales = append(locales, localization.SupportedLocales...)
	} else {
		locale, err := localization.Parse(*localeFlag)
		if err != nil {
			fatal(err)
		}
		locales = append(locales, locale)
	}
	for _, locale := range locales {
		if err := generateLocale(filepath.Join(*dir, string(locale)), locale); err != nil {
			fatal(err)
		}
	}
}

func generateLocale(dir string, locale localization.Locale) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	text := demoContent(locale)
	left := leftSheets(text)
	right := rightSheets(text)
	positionLeft := leftSheets(text)
	positionRight := rightSheets(text)
	positionLeft[0].rows[0][0] = text.positionHeader
	positionRight[0].rows[0][0] = text.positionHeader
	merged := leftSheets(text)
	merged[0].rows[1][4] = 1360

	files := map[string][]sheetData{
		"balance_worktree.xlsx":       left,
		"balance_origin-main.xlsx":    right,
		"balance_position-left.xlsx":  positionLeft,
		"balance_position-right.xlsx": positionRight,
		"balance_merged-left.xlsx":    merged,
	}
	for name, sheets := range files {
		if err := writeWorkbook(filepath.Join(dir, name), sheets, text.sheets[0], locale); err != nil {
			return err
		}
	}
	fmt.Printf("generated %s product demo in %s\n", locale, dir)
	return nil
}

func demoContent(locale localization.Locale) demoText {
	switch locale {
	case localization.SimplifiedChinese:
		return demoText{
			sheets: [3]string{"角色成长", "关卡掉落", "技能参数"}, positionHeader: "序号",
			characterHeader: []any{"id", "role_key", "角色名称", "定位", "生命值", "攻击力", "防御力", "暴击率", "移动速度", "解锁等级", "资源路径"},
			stageHeader:     []any{"id", "stage_key", "关卡名称", "体力消耗", "金币", "经验值", "首通道具", "首通数量", "常规掉落", "掉落权重"},
			skillHeader:     []any{"id", "skill_key", "技能名称", "伤害系数", "冷却时间（秒）", "能量消耗", "目标规则", "效果资源"},
			characters:      [11]string{"铁卫·洛恩", "苍羽·艾拉", "星术师·诺亚", "圣愈·米娅", "赤刃·卡恩", "霜语·伊芙", "岩心·格罗姆", "夜巡·希娅", "雷铳·维克", "森灵·芙蕾", "潮汐·伊澜"},
			roles:           [6]string{"坦克", "游侠", "法师", "辅助", "战士", "刺客"},
			stages:          [9]string{"雾林外围", "古树回廊", "荆棘守望者", "沉没遗迹", "回声穹顶", "无声构装体", "霜原前哨", "白夜峡谷", "凛冬巨像"},
			skills:          [6]string{"壁垒冲击", "穿云箭", "星陨", "圣愈潮汐", "赤刃连斩", "霜环"},
		}
	case localization.Japanese:
		return demoText{
			sheets: [3]string{"キャラクター成長", "ステージ報酬", "スキル設定"}, positionHeader: "行番号",
			characterHeader: []any{"id", "role_key", "表示名", "ロール", "HP", "攻撃力", "防御力", "クリティカル率", "移動速度", "解放レベル", "アセットパス"},
			stageHeader:     []any{"id", "stage_key", "ステージ名", "消費スタミナ", "ゴールド", "経験値", "初回報酬", "初回個数", "通常ドロップ", "ドロップ重み"},
			skillHeader:     []any{"id", "skill_key", "スキル名", "ダメージ倍率", "クールダウン（秒）", "消費エネルギー", "対象ルール", "エフェクトパス"},
			characters:      [11]string{"ガレス", "エリア", "ノクス", "ミレイユ", "カイル", "イヴェル", "グレン", "シア", "ヴィクター", "フレイア", "イラーナ"},
			roles:           [6]string{"タンク", "レンジャー", "メイジ", "サポート", "ファイター", "アサシン"},
			stages:          [9]string{"霧の森・外縁", "古樹の回廊", "茨の番人", "水没遺跡", "残響の天蓋", "沈黙の機兵", "霜原の前哨地", "白夜の峡谷", "冬嵐の巨像"},
			skills:          [6]string{"シールドバッシュ", "蒼天の矢", "スターフォール", "セイクリッドタイド", "クリムゾンエッジ", "フロストリング"},
		}
	default:
		return demoText{
			sheets: [3]string{"Character Growth", "Stage Rewards", "Skill Tuning"}, positionHeader: "Row Number",
			characterHeader: []any{"id", "role_key", "Display Name", "Role", "HP", "Attack", "Defense", "Critical Rate", "Move Speed", "Unlock Level", "Asset Path"},
			stageHeader:     []any{"id", "stage_key", "Stage Name", "Stamina Cost", "Gold", "Experience", "First-Clear Item", "First-Clear Quantity", "Standard Drop", "Drop Weight"},
			skillHeader:     []any{"id", "skill_key", "Skill Name", "Damage Multiplier", "Cooldown (sec)", "Energy Cost", "Target Rule", "Effect Asset Path"},
			characters:      [11]string{"Gareth", "Ayla", "Nox", "Mira", "Kael", "Ivelle", "Grom", "Sia", "Viktor", "Freya", "Ilara"},
			roles:           [6]string{"Tank", "Ranger", "Mage", "Support", "Fighter", "Assassin"},
			stages:          [9]string{"Mistwood Outskirts", "Elderbough Passage", "Thornwatch", "Sunken Ruins", "Echo Vault", "Silent Automaton", "Frostfield Outpost", "White Night Ravine", "Winter Colossus"},
			skills:          [6]string{"Shield Bash", "Sky-Piercing Arrow", "Starfall", "Sacred Tide", "Crimson Edge", "Frost Ring"},
		}
	}
}

func leftSheets(text demoText) []sheetData {
	return []sheetData{
		{name: text.sheets[0], rows: [][]any{
			append([]any(nil), text.characterHeader...),
			{1001, "guardian_lorne", text.characters[0], text.roles[0], 1280, 72, 118, 0.05, 4.2, 1, "Role/Guardian/Lorne"},
			{1002, "ranger_ayla", text.characters[1], text.roles[1], 860, 142, 54, 0.18, 5.1, 1, "Role/Ranger/Ayla"},
			{1003, "mage_noah", text.characters[2], text.roles[2], 790, 158, 46, 0.12, 4.6, 5, "Role/Mage/Noah"},
			{1004, "healer_mia", text.characters[3], text.roles[3], 920, 96, 70, 0.08, 4.8, 8, "Role/Healer/Mia"},
			{1005, "fighter_karn", text.characters[4], text.roles[4], 1080, 132, 82, 0.15, 4.7, 10, "Role/Fighter/Karn"},
			{1006, "frost_eve", text.characters[5], text.roles[2], 810, 151, 49, 0.13, 4.5, 12, "Role/Mage/Eve"},
			{1007, "warden_grom", text.characters[6], text.roles[0], 1450, 65, 134, 0.04, 3.9, 15, "Role/Guardian/Grom"},
			{1008, "assassin_sia", text.characters[7], text.roles[5], 760, 176, 44, 0.24, 5.4, 18, "Role/Assassin/Sia"},
			{1009, "gunner_vick", text.characters[8], text.roles[1], 850, 164, 52, 0.20, 5.0, 20, "Role/Gunner/Vick"},
			{1010, "druid_frey", text.characters[9], text.roles[3], 900, 102, 66, 0.10, 4.9, 22, "Role/Healer/Frey"},
		}},
		{name: text.sheets[1], rows: [][]any{
			append([]any(nil), text.stageHeader...),
			{2101, "forest_01", text.stages[0], 6, 420, 120, "gem", 30, "wood_shard", 100},
			{2102, "forest_02", text.stages[1], 6, 460, 132, "stamina", 20, "wood_shard", 100},
			{2103, "forest_boss", text.stages[2], 10, 880, 260, "hero_token", 5, "guardian_core", 35},
			{2201, "ruins_01", text.stages[3], 8, 560, 168, "gem", 35, "aether_dust", 100},
			{2202, "ruins_02", text.stages[4], 8, 610, 182, "stamina", 25, "aether_dust", 100},
			{2203, "ruins_boss", text.stages[5], 12, 1120, 330, "hero_token", 6, "machine_core", 30},
			{2301, "snow_01", text.stages[6], 10, 720, 220, "gem", 40, "frost_crystal", 100},
			{2302, "snow_02", text.stages[7], 10, 780, 238, "stamina", 30, "frost_crystal", 100},
			{2303, "snow_boss", text.stages[8], 14, 1380, 410, "hero_token", 8, "giant_heart", 25},
		}},
		{name: text.sheets[2], rows: [][]any{
			append([]any(nil), text.skillHeader...),
			{3101, "shield_bash", text.skills[0], 1.35, 8, 20, "nearest_enemy", "Fx/Skill/ShieldBash"},
			{3102, "wind_arrow", text.skills[1], 1.82, 6, 18, "lowest_hp_enemy", "Fx/Skill/WindArrow"},
			{3103, "star_fall", text.skills[2], 2.45, 14, 45, "enemy_cluster", "Fx/Skill/StarFall"},
			{3104, "holy_tide", text.skills[3], 0.95, 12, 38, "lowest_hp_ally", "Fx/Skill/HolyTide"},
			{3105, "red_arc", text.skills[4], 2.10, 10, 32, "front_row", "Fx/Skill/RedArc"},
			{3106, "frost_ring", text.skills[5], 1.58, 11, 34, "enemy_cluster", "Fx/Skill/FrostRing"},
		}},
	}
}

func rightSheets(text demoText) []sheetData {
	sheets := leftSheets(text)
	sheets[0].rows[1][2] = map[localization.Locale]string{localization.English: "Gareth the Bulwark", localization.SimplifiedChinese: "铁壁·洛恩", localization.Japanese: "鉄壁のガレス"}[localeForText(text)]
	sheets[0].rows[1][4] = 1360
	sheets[0].rows[2][5] = 150
	sheets[0].rows[3][9] = 3
	sheets[0].rows[4][8] = 5.0
	newRole := []any{1011, "tide_ilan", text.characters[10], text.roles[3], 940, 108, 68, 0.09, 4.8, 24, "Role/Healer/Ilan"}
	sheets[0].rows = append(sheets[0].rows, nil)
	copy(sheets[0].rows[5:], sheets[0].rows[4:])
	sheets[0].rows[4] = newRole
	sheets[1].rows[4][4] = 600
	sheets[1].rows[6][9] = 35
	sheets[1].rows = sheets[1].rows[:len(sheets[1].rows)-1]
	sheets[2].rows[2][3] = 1.92
	sheets[2].rows[3][4] = 13
	sheets[2].rows[5][5] = 30
	return sheets
}

func localeForText(text demoText) localization.Locale {
	switch text.sheets[0] {
	case "角色成长":
		return localization.SimplifiedChinese
	case "キャラクター成長":
		return localization.Japanese
	default:
		return localization.English
	}
}

func writeWorkbook(path string, sheets []sheetData, characterSheet string, locale localization.Locale) error {
	f := excelize.NewFile()
	defer f.Close()
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"17324C"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
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
		if sheet.name == characterSheet {
			if err := f.SetCellStyle(sheet.name, "H2", fmt.Sprintf("H%d", len(sheet.rows)), percentStyle); err != nil {
				return err
			}
		}
		if err := f.SetRowHeight(sheet.name, 1, 24); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "A", "A", 12); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "B", "B", 20); err != nil {
			return err
		}
		nameWidth, secondaryWidth, dataWidth := 18.0, 18.0, 15.0
		switch locale {
		case localization.English:
			nameWidth, secondaryWidth, dataWidth = 22, 18, 17
		case localization.Japanese:
			nameWidth, secondaryWidth, dataWidth = 20, 17, 18
		}
		if err := f.SetColWidth(sheet.name, "C", "C", nameWidth); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "D", "D", secondaryWidth); err != nil {
			return err
		}
		if err := f.SetColWidth(sheet.name, "E", lastColumn, dataWidth); err != nil {
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

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
