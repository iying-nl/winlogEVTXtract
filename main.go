package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"
)

type Event struct {
    EventId int               `json:"EventId"`
    Payload string            `json:"Payload"`
    Data    map[string]string `json:"-"`
}

var targetEventIds = []int{
    4624, 4625, 4634, 4647,
    4720, 4726, 4724, 4740, 4722, 4776,4723, 4725, 4732,
    4738, 4741, 4742, 4743,
    1102,
}

func main() {
    start := time.Now()

    currentDir, err := os.Getwd()
    if err != nil {
        fmt.Printf("获取当前目录失败: %v\n", err)
        return
    }

    evtxECmdPath := filepath.Join(currentDir, "EvtxECmd.exe")
    if _, err := os.Stat(evtxECmdPath); os.IsNotExist(err) {
        fmt.Printf("EvtxECmd.exe 不存在于当前目录: %s\n", evtxECmdPath)
        return
    }

    inputDir := "raw_evtx_logs"
    outputDir := "converted_json"

    if _, err := os.Stat(inputDir); os.IsNotExist(err) {
        fmt.Printf("输入目录 %s 不存在\n", inputDir)
        return
    }

    if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
        fmt.Printf("创建输出目录 %s 失败: %v\n", outputDir, err)
        return
    }

    fileChan := make(chan string)
    var wg sync.WaitGroup

    maxWorkers := 4
    for i := 0; i < maxWorkers; i++ {
        wg.Add(1)
        go worker(evtxECmdPath, outputDir, fileChan, &wg)
    }

    go func() {
        err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return err
            }
            if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".evtx") {
                fileChan <- path
            }
            return nil
        })
        if err != nil {
            fmt.Printf("遍历目录时出错: %v\n", err)
        }
        close(fileChan)
    }()

    wg.Wait()

    files, err := filepath.Glob(filepath.Join(outputDir, "*.json"))
    if err != nil {
        fmt.Println("错误：无法获取JSON文件列表:", err)
        return
    }

    if len(files) == 0 {
        fmt.Printf("错误：%s 目录下没有找到JSON文件。\n", outputDir)
        return
    }

    fmt.Printf("开始处理 %d 个文件...\n", len(files))
    processFiles(files)

    fmt.Println("\n开始合并同类事件文件...")
    if err := mergeEventFiles(); err != nil {
        fmt.Printf("合并文件时发生错误：%v\n", err)
    }

    fmt.Println("\n开始格式化事件文件...")
    if err := formatEvents(); err != nil {
        fmt.Printf("格式化事件文件时发生错误：%v\n", err)
    }

    fmt.Println("\n开始添加 logip 字段...")
    if err := addLogIPToFormattedEvents(); err != nil {
        fmt.Printf("添加 logip 字段时发生错误：%v\n", err)
    }

    duration := time.Since(start)
    fmt.Printf("\n所有处理完成，总耗时: %v\n", duration)
}

func worker(evtxECmdPath, outputDir string, fileChan <-chan string, wg *sync.WaitGroup) {
    defer wg.Done()
    for inputFile := range fileChan {
        outputFile := filepath.Join(outputDir, filepath.Base(inputFile)+".json")

        cmd := exec.Command(evtxECmdPath,
            "-f", inputFile,
            "--json", outputDir,
            "--jsonf", filepath.Base(outputFile))

        output, err := cmd.CombinedOutput()
        if err != nil {
            fmt.Printf("处理文件 %s 时出错: %v\n", inputFile, err)
            fmt.Println(string(output))
        } else {
            fmt.Printf("成功处理文件: %s\n", inputFile)
        }
    }
}

func parsePayload(payload string) map[string]string {
    var payloadData struct {
        EventData struct {
            Data []struct {
                Name  string `json:"@Name"`
                Value string `json:"#text"`
            } `json:"Data"`
        } `json:"EventData"`
    }

    err := json.Unmarshal([]byte(payload), &payloadData)
    if err != nil {
        return nil
    }

    result := make(map[string]string)
    for _, item := range payloadData.EventData.Data {
        result[item.Name] = item.Value
    }
    return result
}

func processFiles(files []string) {
    sem := make(chan struct{}, 10) // 限制并发数为10
    var wg sync.WaitGroup

    for _, file := range files {
        wg.Add(1)
        go func(file string) {
            defer wg.Done()
            sem <- struct{}{} // 获取信号量
            defer func() { <-sem }() // 释放信号量

            if err := processFile(file); err != nil {
                fmt.Printf("处理文件 %s 时发生错误：%v\n", file, err)
            }
        }(file)
    }

    wg.Wait()
}

func processFile(inputFile string) error {
    file, err := os.Open(inputFile)
    if err != nil {
        return err
    }
    defer file.Close()

    if err := os.MkdirAll("filtered_events", os.ModePerm); err != nil {
        return fmt.Errorf("无法创建 filtered_events 文件夹: %v", err)
    }

    eventCounts := make(map[int]int)
    filteredEvents := make(map[int][]map[string]interface{})

    reader := bufio.NewReader(file)
    bom, _ := reader.Peek(3)
    if bytes.Equal(bom, []byte{0xEF, 0xBB, 0xBF}) {
        reader.Discard(3)
    }

    lineNumber := 0
    for {
        line, err := reader.ReadBytes('\n')
        if err != nil && err != io.EOF {
            return err
        }

        lineNumber++
        if len(bytes.TrimSpace(line)) == 0 {
            if err == io.EOF {
                break
            }
            continue
        }

        var event Event
        if err := json.Unmarshal(line, &event); err != nil {
            if err == io.EOF {
                break
            }
            continue
        }

        if contains(targetEventIds, event.EventId) {
            eventMap := make(map[string]interface{})
            var tempMap map[string]interface{}
            json.Unmarshal(line, &tempMap)
            for k, v := range tempMap {
                if k != "Payload" {
                    eventMap[k] = v
                }
            }

            payloadData := parsePayload(event.Payload)
            for k, v := range payloadData {
                eventMap[k] = v
            }

            filteredEvents[event.EventId] = append(filteredEvents[event.EventId], eventMap)
            eventCounts[event.EventId]++
        }

        if err == io.EOF {
            break
        }
    }

    baseFileName := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))

    totalEvents := 0
    for eventId, events := range filteredEvents {
        outputFile := filepath.Join("filtered_events", fmt.Sprintf("%s_%d_parsed.json", baseFileName, eventId))
        outputData, err := json.MarshalIndent(events, "", "  ")
        if err != nil {
            return err
        }

        err = ioutil.WriteFile(outputFile, outputData, 0644)
        if err != nil {
            return err
        }

        totalEvents += len(events)
    }

    fmt.Printf("处理完成: %s (共 %d 个事件)\n", inputFile, totalEvents)

    return nil
}

func contains(slice []int, item int) bool {
    for _, a := range slice {
        if a == item {
            return true
        }
    }
    return false
}

func mergeEventFiles() error {
    if err := os.MkdirAll("merged_events", os.ModePerm); err != nil {
        return fmt.Errorf("无法创建 merged_events 文件夹: %v", err)
    }

    files, err := filepath.Glob("filtered_events/*_parsed.json")
    if err != nil {
        return fmt.Errorf("无法获取filtered_events目录下的文件: %v", err)
    }

    eventMap := make(map[int][]map[string]interface{})

    for _, file := range files {
        data, err := ioutil.ReadFile(file)
        if err != nil {
            fmt.Printf("读取文件 %s 时发生错误: %v\n", file, err)
            continue
        }

        var events []map[string]interface{}
        if err := json.Unmarshal(data, &events); err != nil {
            fmt.Printf("解析文件 %s 时发生错误: %v\n", file, err)
            continue
        }

        fileName := filepath.Base(file)
        eventIdStr := strings.Split(fileName, "_")[1]
        eventId := 0
        fmt.Sscanf(eventIdStr, "%d", &eventId)

        eventMap[eventId] = append(eventMap[eventId], events...)
    }

    totalMergedEvents := 0
    for eventId, events := range eventMap {
        outputFile := filepath.Join("merged_events", fmt.Sprintf("%d_parsed.json", eventId))
        outputData, err := json.MarshalIndent(events, "", "  ")
        if err != nil {
            fmt.Printf("序列化事件 %d 时发生错误: %v\n", eventId, err)
            continue
        }

        if err := ioutil.WriteFile(outputFile, outputData, 0644); err != nil {
            fmt.Printf("写入文件 %s 时发生错误: %v\n", outputFile, err)
            continue
        }

        totalMergedEvents += len(events)
    }

    fmt.Printf("合并完成: 共处理 %d 个事件\n", totalMergedEvents)

    return nil
}

func formatEvents() error {
    inputFolder := "merged_events"
    outputFolder := "formatted_events"

    err := os.MkdirAll(outputFolder, os.ModePerm)
    if err != nil {
        return fmt.Errorf("创建输出文件夹失败: %v", err)
    }

    files, err := ioutil.ReadDir(inputFolder)
    if err != nil {
        return fmt.Errorf("读取文件夹失败: %v", err)
    }

    fmt.Printf("开始格式化文件...\n")

    var wg sync.WaitGroup
    sem := make(chan struct{}, 10) // 限制并发数为10
    successCount := 0
    failCount := 0
    var mu sync.Mutex

    for _, file := range files {
        if filepath.Ext(file.Name()) == ".json" {
            wg.Add(1)
            go func(file os.FileInfo) {
                defer wg.Done()
                sem <- struct{}{} // 获取信号量
                defer func() { <-sem }() // 释放信号量

                inputPath := filepath.Join(inputFolder, file.Name())
                outputPath := filepath.Join(outputFolder, file.Name())
                if processJSONFile(inputPath, outputPath) {
                    mu.Lock()
                    successCount++
                    mu.Unlock()
                    fmt.Printf(".")
                } else {
                    mu.Lock()
                    failCount++
                    mu.Unlock()
                    fmt.Printf("x")
                }
            }(file)
        }
    }

    wg.Wait()

    fmt.Printf("\n格式化完成。成功: %d, 失败: %d\n", successCount, failCount)
    return nil
}

func processJSONFile(inputPath, outputPath string) bool {
    content, err := ioutil.ReadFile(inputPath)
    if err != nil {
        return false
    }

    var dataArray []map[string]interface{}
    err = json.Unmarshal(content, &dataArray)
    if err != nil {
        return false
    }

    for i, data := range dataArray {
        for key, value := range data {
            switch v := value.(type) {
            case float64:
                dataArray[i][key] = strconv.FormatFloat(v, 'f', -1, 64)
            case bool:
                dataArray[i][key] = strconv.FormatBool(v)
            case nil:
                dataArray[i][key] = ""
            default:
                dataArray[i][key] = fmt.Sprintf("%v", v)
            }
        }
    }

    newContent, err := json.MarshalIndent(dataArray, "", "  ")
    if err != nil {
        return false
    }

    err = ioutil.WriteFile(outputPath, newContent, 0644)
    if err != nil {
        return false
    }

    return true
}

func addLogIPToFormattedEvents() error {
    inputFolder := "formatted_events"
    outputFolder := "final_events"

    err := os.MkdirAll(outputFolder, os.ModePerm)
    if err != nil {
        return fmt.Errorf("创建输出文件夹失败: %v", err)
    }

    files, err := ioutil.ReadDir(inputFolder)
    if err != nil {
        return fmt.Errorf("读取文件夹失败: %v", err)
    }

    fmt.Printf("开始添加 logip 字段...\n")

    var wg sync.WaitGroup
    sem := make(chan struct{}, 10) // 限制并发数为10
    successCount := 0
    failCount := 0
    var mu sync.Mutex

    for _, file := range files {
        if filepath.Ext(file.Name()) == ".json" {
            wg.Add(1)
            go func(file os.FileInfo) {
                defer wg.Done()
                sem <- struct{}{} // 获取信号量
                defer func() { <-sem }() // 释放信号量

                inputPath := filepath.Join(inputFolder, file.Name())
                outputPath := filepath.Join(outputFolder, file.Name())
                if processJSONFileAddLogIP(inputPath, outputPath) {
                    mu.Lock()
                    successCount++
                    mu.Unlock()
                    fmt.Printf(".")
                } else {
                    mu.Lock()
                    failCount++
                    mu.Unlock()
                    fmt.Printf("x")
                }
            }(file)
        }
    }

    wg.Wait()

    fmt.Printf("\n处理完成。成功: %d, 失败: %d\n", successCount, failCount)
    return nil
}

func processJSONFileAddLogIP(inputPath, outputPath string) bool {
    content, err := ioutil.ReadFile(inputPath)
    if err != nil {
        return false
    }

    var dataArray []map[string]interface{}
    err = json.Unmarshal(content, &dataArray)
    if err != nil {
        return false
    }

    for i, data := range dataArray {
        if sourceFile, ok := data["SourceFile"].(string); ok {
            ip := extractIPFromSourceFile(sourceFile)
            if ip != "" {
                dataArray[i]["logip"] = ip
            }
        }
    }

    newContent, err := json.MarshalIndent(dataArray, "", "  ")
    if err != nil {
        return false
    }

    err = ioutil.WriteFile(outputPath, newContent, 0644)
    if err != nil {
        return false
    }

    return true
}

func extractIPFromSourceFile(sourceFile string) string {
    parts := strings.Split(sourceFile, "\\")
    if len(parts) > 0 {
        fileName := parts[len(parts)-1]
        ipPart := strings.TrimSuffix(fileName, ".evtx")
        if net.ParseIP(ipPart) != nil {
            return ipPart
        }
    }
    return ""
}
