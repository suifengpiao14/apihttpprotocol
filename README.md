# 接口协议 apihttpprotocol

公司接口协议存在一层协议,二层协议 以及各种一、二层协议的变种，比较混乱。
封装此库统一方面统一兼容处理各种协议，另一方面推广标准协议
具体功能需求：
1. 收到各种协议的json请求体，能Decode业务数据供业务逻辑处理，处理完能按指定协议Encode返回数据
2. 收到数据、输出数据时支持打印日志
3. 能记录请求耗时
4. 能统一处理http请求错误，统一返回错误信息

**公司标准一层协议:**
1. 请求体必须是json格式，并以application/json 方式传输

```
{
    "pageSize" : "10",
    "pageIndex" : "0",
    "orderId":"12",
    "login_token" : "::765"
}
```
2. 响应体必须是json格式，并以application/json 方式传输
```
{
    "_errCode": "0",
    "_errStr": "SUCCEED",
    "_ret": "0",
    "_data"{
        // 业务数据
    }
}
```

**公司标准二层协议:**
1. 请求体必须是json格式，并以application/json 方式传输
```
{
  "_head":{
    "_version":"0.01",
    "_msgType":"request",
    "_timestamps":"1523330331",
    "_invokeId":"book1523330331358",
    "_callerServiceId":"110001",
    "_groupNo":"1",
    "_interface":"efence.admin.efenceUpdate",
    "_remark":""
  },
  "_param":{
// 业务参数
}
}
```
2. 响应体必须是json格式，并以application/json 方式传输
```
{
    "_head": {
        "_interface": "heatmap.api.createPoint",
        "_msgType": "response",
        "_remark": "",
        "_version": "0.01",
        "_timestamps": "1602488688",
        "_invokeId": "",
        "_callerServiceId": "110026",
        "_groupNo": "1"
    },
    "_data": {
        "_ret": "0",
        "_errCode": "0",
        "_errStr": "success",
        "_data":{
             // 业务数据
        }
    }
}
```

**公司二层变种协议:**
1. 返回体缺少一层_data
```
{
    "_head": {
        "_interface": "heatmap.api.createPoint",
        "_msgType": "response",
        "_remark": "",
        "_version": "0.01",
        "_timestamps": "1602488688",
        "_invokeId": "",
        "_callerServiceId": "110026",
        "_groupNo": "1"
    },
    "_data": {
        "_ret": "0",
        "_errCode": "0",
        "_errStr": "success",
         // 业务数据
    }
}
```
2. 返回体 body ret retcode retinfo
```
{
    "_head": {
        "_interface": "heatmap.api.createPoint",
        "_msgType": "response",
        "_remark": "",
        "_version": "0.01",
        "_timestamps": "1602488688",
        "_invokeId": "",
        "_callerServiceId": "110026",
        "_groupNo": "1"
    },
    "_data": {
        "ret": "0",
        "retcode": "0",
        "retinfo": "success",
        "body":{
             // 业务数据
         }
    }
}
```
3. 返回体 _body _ret _retcode _retinfo
```
{
    "_head": {
        "_interface": "heatmap.api.createPoint",
        "_msgType": "response",
        "_remark": "",
        "_version": "0.01",
        "_timestamps": "1602488688",
        "_invokeId": "",
        "_callerServiceId": "110026",
        "_groupNo": "1"
    },
    "_data": {
        "_ret": "0",
        "_retcode": "0",
        "_retinfo": "success",
        "_body":{
             // 业务数据
         }
    }
}
```