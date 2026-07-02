package main

import (
	"fmt"
	"strings"
)

// ---------- главная деталь кузова (<modId>.jbeam) ----------

func tplMainJbeam(p tplParams) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	c := func(en, ru string) string { return "// " + cm(p.Lang, en, ru) }

	w("{")
	w(fmt.Sprintf("%q: {", p.ModID))
	w("    " + c("Main body part. slotType \"main\" makes it the root part that is spawned directly.",
		"Главная деталь кузова. slotType \"main\" делает её корневой - её спавнит игра."))
	w("    \"information\": {")
	w(fmt.Sprintf("        \"name\": %q,          %s", p.Name, c("shown in the vehicle selector", "показывается в списке машин")))
	w(fmt.Sprintf("        \"authors\": %q,", p.Author))
	w("        \"value\": 1000,             " + c("in-game price", "цена в игре"))
	w("    },")
	w("    \"slotType\": \"main\",")
	w("")
	w("    " + c("Slots: child parts plug in here by matching slotType.",
		"Слоты: сюда подключаются дочерние детали по совпадению slotType."))
	w("    \"slots\": [")
	w("        [\"type\", \"default\", \"description\"],")
	w(fmt.Sprintf("        [%q, %q, \"Engine\"],", p.ModID+"_engine", p.ModID+"_engine"))
	w("    ],")
	w("")
	w("    " + c("refNodes set the vehicle orientation (values are node ids below).",
		"refNodes задают ориентацию машины (значения - id узлов ниже)."))
	w("    \"refNodes\": [")
	w("        [\"ref:\", \"back:\", \"left:\", \"up:\"],")
	w("        [\"ref1\", \"ref2\", \"ref3\", \"ref4\"],")
	w("    ],")
	w("")
	w("    " + c("Nodes = point masses. Coords are meters: +X left, +Y back, +Z up (forward = -Y).",
		"Узлы = точечные массы. Координаты в метрах: +X влево, +Y назад, +Z вверх (вперёд = -Y)."))
	w("    \"nodes\": [")
	w("        [\"id\", \"posX\", \"posY\", \"posZ\"],")
	w("        {\"nodeWeight\": 25},        " + c("weight applies to the nodes below", "вес применяется к узлам ниже"))
	w("        " + c("Simple box body - replace with your real geometry.",
		"Простой ящик-кузов - замените на свою реальную геометрию."))
	w("        [\"b1\", -0.7, -1.5, 0.4],")
	w("        [\"b2\",  0.7, -1.5, 0.4],")
	w("        [\"b3\", -0.7,  1.5, 0.4],")
	w("        [\"b4\",  0.7,  1.5, 0.4],")
	w("        [\"b5\", -0.7, -1.5, 1.0],")
	w("        [\"b6\",  0.7, -1.5, 1.0],")
	w("        [\"b7\", -0.7,  1.5, 1.0],")
	w("        [\"b8\",  0.7,  1.5, 1.0],")
	w("        " + c("Reference nodes for orientation.", "Опорные узлы для ориентации."))
	w("        [\"ref1\", 0.0, 0.0, 0.4],")
	w("        [\"ref2\", 0.0, 1.0, 0.4],")
	w("        [\"ref3\", 1.0, 0.0, 0.4],")
	w("        [\"ref4\", 0.0, 0.0, 1.4],")
	w("    ],")
	w("")
	w("    " + c("Beams = springs between two nodes.", "Балки = пружины между двумя узлами."))
	w("    \"beams\": [")
	w("        [\"id1:\", \"id2:\"],")
	w("        {\"beamSpring\": 3000000, \"beamDamp\": 100, \"beamDeform\": 50000, \"beamStrength\": 200000},")
	w("        " + c("Box edges.", "Рёбра ящика."))
	w("        [\"b1\",\"b2\"], [\"b3\",\"b4\"], [\"b1\",\"b3\"], [\"b2\",\"b4\"],")
	w("        [\"b5\",\"b6\"], [\"b7\",\"b8\"], [\"b5\",\"b7\"], [\"b6\",\"b8\"],")
	w("        [\"b1\",\"b5\"], [\"b2\",\"b6\"], [\"b3\",\"b7\"], [\"b4\",\"b8\"],")
	w("        " + c("Diagonals - bracing is what keeps the structure rigid.",
		"Диагонали - именно они держат жёсткость конструкции."))
	w("        [\"b1\",\"b4\"], [\"b5\",\"b8\"], [\"b1\",\"b8\"], [\"b2\",\"b7\"],")
	w("    ],")
	w("")
	w("    " + c("Triangles = surfaces for collision and aerodynamics (3 nodes each).",
		"Треугольники = поверхности для столкновений и аэродинамики (по 3 узла)."))
	w("    \"triangles\": [")
	w("        [\"id1:\", \"id2:\", \"id3:\"],")
	w("        [\"b5\",\"b6\",\"b8\"],")
	w("        [\"b5\",\"b8\",\"b7\"],")
	w("    ],")
	w("")
	w("    " + c("Flexbodies bind a .dae mesh to the nodes. Uncomment when you have a mesh exported from Blender.",
		"Flexbodies привязывают меш .dae к узлам. Раскомментируйте, когда экспортируете меш из Blender."))
	w("    \"flexbodies\": [")
	w("        [\"mesh\", \"[group]:\", \"nonFlexMaterials\"],")
	w(fmt.Sprintf("        // [%q, [%q]],", p.ModID+"_body", p.ModID+"_body"))
	w("    ],")
	w("}")
	w("}")
	return b.String()
}

// ---------- деталь двигателя (<modId>_engine.jbeam) ----------

func tplEngineJbeam(p tplParams) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	c := func(en, ru string) string { return "// " + cm(p.Lang, en, ru) }

	w("{")
	w(fmt.Sprintf("%q: {", p.ModID+"_engine"))
	w("    " + c("Engine part. Its slotType must match the slot declared in the main part.",
		"Деталь двигателя. Её slotType должен совпадать со слотом в главной детали."))
	w("    \"information\": {")
	w("        \"name\": \"Engine\",")
	w(fmt.Sprintf("        \"authors\": %q,", p.Author))
	w("    },")
	w(fmt.Sprintf("    \"slotType\": %q,", p.ModID+"_engine"))
	w("")
	w("    " + c("Simplified engine for learning. The torque curve [rpm, Nm] sets the power.",
		"Упрощённый двигатель для обучения. Кривая момента [обороты, Нм] задаёт мощность."))
	w("    " + c("The LAST rpm point is the real rev limit in game - add points to rev higher.",
		"ПОСЛЕДНЯЯ точка оборотов - реальный предел раскрутки в игре, добавьте точки, чтобы крутить выше."))
	w("    \"engine\": {")
	w("        \"torque\": [")
	w("            [\"rpm\", \"torque\"],")
	w("            [0, 0],")
	w("            [1000, 120],")
	w("            [3000, 200],")
	w("            [5000, 230],")
	w("            [6500, 210],")
	w("            [7000, 0],")
	w("        ],")
	w("        \"idleRPM\": 900,")
	w("        \"maxRPM\": 7000,")
	w("        \"inertia\": 0.15,")
	w("        \"friction\": 20,")
	w("    },")
	w("    " + c("Tip: the app's \"Constructor\" tab can build a full, valid engine jbeam for you.",
		"Совет: вкладка «Конструктор» в программе соберёт полноценный валидный jbeam двигателя."))
	w("}")
	w("}")
	return b.String()
}

// ---------- info.json (карточка машины в списке) ----------

func tplInfoJSON(p tplParams) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	c := func(en, ru string) string { return "// " + cm(p.Lang, en, ru) }

	w("{")
	w("    " + c("Vehicle entry shown in the car selector.", "Карточка машины в списке выбора."))
	w(fmt.Sprintf("    \"name\": %q,", p.Name))
	w(fmt.Sprintf("    \"brand\": %q,", p.Author))
	w("    \"type\": \"Car\",")
	w(fmt.Sprintf("    \"default_pc\": %q,   %s", p.ModID, c("configuration loaded by default", "конфигурация по умолчанию")))
	w(fmt.Sprintf("    \"author\": %q,", p.Author))
	w("}")
	return b.String()
}

// ---------- .pc (конфигурация: какие детали в слотах) ----------

func tplConfigPC(p tplParams) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	c := func(en, ru string) string { return "// " + cm(p.Lang, en, ru) }

	w("{")
	w("    " + c("A configuration: which part fills each slot, plus paint and variables.",
		"Конфигурация: какая деталь стоит в каждом слоте, плюс покраска и переменные."))
	w("    \"format\": 2,")
	w(fmt.Sprintf("    \"model\": %q,", p.ModID))
	w("    \"parts\": {")
	w(fmt.Sprintf("        %q: %q,", p.ModID+"_engine", p.ModID+"_engine"))
	w("    },")
	w("    \"vars\": {},")
	w("}")
	return b.String()
}

// ---------- README.md ----------

func tplREADME(p tplParams) string {
	veh := "vehicles/" + p.ModID + "/"
	if p.Lang == "ru" {
		return fmt.Sprintf(`# %s

Базовый шаблон мода машины для BeamNG.drive, сгенерированный WidthCorrector.

> Это стартовый скелет для обучения структуре мода, а не готовая рабочая машина.
> Здесь нет 3D-меша, колёс и полноценной геометрии - их нужно добавить самому.

## Структура

`+"```"+`
%s
├── README.md                     этот файл
└── vehicles/
    └── %s/
        ├── %s.jbeam        главная деталь кузова (nodes, beams, triangles, slots)
        ├── %s_engine.jbeam деталь двигателя (кривая момента, параметры)
        ├── info.json                 карточка машины в списке выбора
        └── %s.pc           конфигурация (детали в слотах)
`+"```"+`

## Что за файлы

- **%s.jbeam** - главная деталь. slotType "main" делает её корневой. Внутри: узлы
  (nodes - точечные массы), балки (beams - пружины), треугольники (triangles - поверхности),
  слоты (slots - точки подключения дочерних деталей) и flexbodies (привязка 3D-меша).
- **%s_engine.jbeam** - двигатель. Его slotType совпадает со слотом в главной детали.
- **info.json** - имя, бренд, тип, конфигурация по умолчанию (для списка машин).
- **%s.pc** - конфигурация: какая деталь стоит в каждом слоте.

## Система координат

BeamNG использует Z-up и метры: **+X влево, +Y назад, +Z вверх** (вперёд = -Y).

## Как установить

1. Скопируйте папку `+"`"+`%s`+"`"+` (папку `+"`"+`vehicles`+"`"+`) в
   `+"`"+`%%localappdata%%\BeamNG.drive\<версия>\mods\unpacked\<имя_мода>\`+"`"+`
2. Запустите игру, включите режим разработчика и найдите машину в списке.
3. Смотрите консоль игры на ошибки jbeam.

## Дальше

- Добавьте реальный меш (.dae из Blender) и раскомментируйте flexbodies.
- Добавьте подвеску, колёса и правильную геометрию узлов.
- Полноценный двигатель удобно собрать во вкладке «Конструктор» в WidthCorrector.

## Документация

Полная официальная документация: https://documentation.beamng.com/modding/vehicle/
`,
			p.Name, p.FolderPath, p.ModID, p.ModID, p.ModID, p.ModID,
			p.ModID, p.ModID, p.ModID, veh)
	}
	return fmt.Sprintf(`# %s

A basic BeamNG.drive car mod template generated by WidthCorrector.

> This is a starter skeleton for learning the mod structure, not a finished working car.
> There is no 3D mesh, no wheels and no full geometry yet - you add those yourself.

## Structure

`+"```"+`
%s
├── README.md                     this file
└── vehicles/
    └── %s/
        ├── %s.jbeam        main body part (nodes, beams, triangles, slots)
        ├── %s_engine.jbeam engine part (torque curve, parameters)
        ├── info.json                 vehicle entry for the selector
        └── %s.pc           a configuration (which part fills each slot)
`+"```"+`

## What the files are

- **%s.jbeam** - the main part. slotType "main" makes it the root. Inside: nodes
  (point masses), beams (springs), triangles (surfaces), slots (where child parts plug in)
  and flexbodies (binding of the 3D mesh).
- **%s_engine.jbeam** - the engine. Its slotType matches the slot in the main part.
- **info.json** - name, brand, type, default configuration (for the car selector).
- **%s.pc** - a configuration: which part fills each slot.

## Coordinate system

BeamNG is Z-up and uses meters: **+X left, +Y back, +Z up** (forward = -Y).

## How to install

1. Copy the `+"`"+`vehicles`+"`"+` folder into
   `+"`"+`%%localappdata%%\BeamNG.drive\<version>\mods\unpacked\<mod_name>\`+"`"+`
2. Launch the game, enable developer mode and find the vehicle in the selector.
3. Watch the in-game console for jbeam errors.

## Next steps

- Add a real mesh (.dae from Blender) and uncomment the flexbodies line.
- Add suspension, wheels and proper node geometry.
- A full engine is easy to build in the "Constructor" tab of WidthCorrector.

## Documentation

Full official docs: https://documentation.beamng.com/modding/vehicle/
`,
		p.Name, p.FolderPath, p.ModID, p.ModID, p.ModID, p.ModID,
		p.ModID, p.ModID, p.ModID)
}
