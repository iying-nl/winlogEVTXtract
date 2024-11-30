
一个使用 Go 语言编写的，用于提取、转换和分析 Windows 事件日志 (EVTX) 的工具。
## 简介
EVTXtract (或你的工具名称)  能够批量处理 Windows 事件日志 (.evtx) 文件，将其转换为结构化的 JSON 数据，并进行过滤、合并和格式化，方便后续分析和使用。

主要功能：

批量转换: 将多个 EVTX 文件转换为 JSON 格式。
IP 提取:  根据 EVTX 文件名提取 IP 地址，并添加到 JSON 数据中。
事件过滤:  根据事件 ID 筛选事件。
事件合并:  将相同事件 ID 的事件合并到一个文件中。
格式化输出:  将 JSON 数据格式化为更易读的形式。
最终结果:  生成包含 `logip` 字段的最终 JSON 文件，方便后续分析和使用。

工作流程:

1. 依赖工具: 使用 `EvtxECmd.exe`  进行初始转换。
2. 原始日志: 从 `raw_evtx_logs` 文件夹读取 EVTX 文件 (文件名格式为 `IP地址.evtx`, 例如 `192.168.1.218.evtx`)。
3. 转换:  将 EVTX 文件转换为原始 JSON 文件。
4. 过滤:  根据预设的事件 ID 列表筛选事件。
5. 合并:  将相同事件 ID 的事件合并。
6. 格式化:  格式化 JSON 数据。
7. 添加 IP:  添加 `logip` 字段到 JSON 数据。
8. 输出:  将最终 JSON 文件保存到 `final_events` 文件夹。

## 依赖

Go 语言环境:  请确保已安装 Go 语言环境 (建议 Go 版本 1.16 或更高)。
EvtxECmd.exe:  需要下载 `EvtxECmd.exe` 并放置在当前目录下。
下载地址:
[https://github.com/EricZimmerman/evtx](https://github.com/EricZimmerman/evtx)
[https://download.ericzimmermanstools.com/net6/EvtxECmd.zip](https://download.ericzimmermanstools.com/net6/EvtxECmd.zip)

## 目录结构

在运行程序之前，请确保创建以下目录和文件结构：
├── EvtxECmd.exe
├── main.go
├── go.mod
├── go.sum
├── raw_evtx_logs
│   ├── 192.168.1.218.evtx
│   └── 192.168.1.219.evtx


## 使用方法

1. 下载依赖:  下载 `EvtxECmd.exe` 并放置在当前目录下。
2. 准备日志:  将 EVTX 文件放置在 `raw_evtx_logs` 文件夹中，文件名格式为 `IP地址.evtx` (例如 `192.168.1.218.evtx`)。
3. 运行程序:
输出:  将最终 JSON 文件保存到 `final_events` 文件夹

##  输出示例

以下是一个 `final_events` 文件夹中 JSON 文件的示例 (例如 `192.168.1.218_4624.json`):

```json
   {
    "AuthenticationPackageName": "Negotiate",
    "Channel": "Security",
    "ChunkNumber": "0",
    "Computer": "WIN-JT0BST00BD4",
    "ElevatedToken": "%%1842",
    "EventId": "4624",
    "EventRecordId": "88726",
    "ExtraDataOffset": "0",
    "HiddenRecord": "false",
    "ImpersonationLevel": "%%1833",
    "IpAddress": "-",
    "IpPort": "-",
    "KeyLength": "0",
    "Keywords": "Audit success",
    "Level": "LogAlways",
    "LmPackageName": "-",
    "LogonGuid": "00000000-0000-0000-0000-000000000000",
    "LogonProcessName": "Advapi  ",
    "LogonType": "5",
    "ProcessId": "0x260",
    "ProcessName": "C:\\Windows\\System32\\services.exe",
    "Provider": "Microsoft-Windows-Security-Auditing",
    "RecordNumber": "2",
    "RestrictedAdminMode": "-",
    "SourceFile": "E:\\开发\\raw_evtx_logs\\192.168.1.218.evtx",
    "SubjectDomainName": "WORKGROUP",
    "SubjectLogonId": "0x3E7",
    "SubjectUserName": "WIN-JT0BST00BD4$",
    "SubjectUserSid": "S-1-5-18",
    "TargetDomainName": "NT AUTHORITY",
    "TargetLinkedLogonId": "0x0",
    "TargetLogonId": "0x3E7",
    "TargetOutboundDomainName": "-",
    "TargetOutboundUserName": "-",
    "TargetUserName": "SYSTEM",
    "TargetUserSid": "S-1-5-18",
    "ThreadId": "748",
    "TimeCreated": "2024-07-08T03:22:38.0757364+00:00",
    "TransmittedServices": "-",
    "VirtualAccount": "%%1843",
    "WorkstationName": "",
    "logip": "192.168.1.218"
  },

