package main

import (
	bili "OPQBot-QQGroupManager/Bili"
	"OPQBot-QQGroupManager/Config"
	"OPQBot-QQGroupManager/GengChaxun"
	"OPQBot-QQGroupManager/androidDns"
	"OPQBot-QQGroupManager/draw"
	"OPQBot-QQGroupManager/githubManager"
	"OPQBot-QQGroupManager/methods"
	"OPQBot-QQGroupManager/utils"
	"OPQBot-QQGroupManager/yiqing"
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	_ "github.com/go-playground/webhooks/v6/github"
	"github.com/mcoo/requests"
	"io/fs"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	"github.com/mcoo/OPQBot"
)

var (
	version = "Ver.0.0.1"
	sess    *sessions.Sessions
)

type WebResult struct {
	Code int         `json:"code"`
	Info string      `json:"info"`
	Data interface{} `json:"data"`
}

//go:embed Web/dist/spa
var staticFs embed.FS

func main() {
	log.Println("QQ Group Manager -️" + version)
	androidDns.SetDns()
	//go CheckUpdate()
	app := iris.New()
	b := OPQBot.NewBotManager(Config.CoreConfig.OPQBotConfig.QQ, Config.CoreConfig.OPQBotConfig.Url)
	err := b.AddEvent(OPQBot.EventNameOnDisconnected, func() {
		log.Println("断开服务器")
	})
	if err != nil {
		log.Println(err)
	}
	g := githubManager.NewManager(app, &b)

	VerifyNum := map[string]*struct {
		Status bool
		Code   string
	}{}
	VerifyLock := sync.Mutex{}
	c := utils.NewBotCronManager()
	c.Start()
	bi := bili.NewManager()
	c.AddJob(-1, "Bili", "*/5 * * * *", func() {
		update, fanju := bi.ScanUpdate()
		for _, v := range update {
			upName, gs ,userId := bi.GetUpGroupsByMid(v.Mid)
			for _, g := range gs {
				if v1, ok := Config.CoreConfig.GroupConfig[g]; ok {
					if !v1.Bili {
						break
					}
				}
				res, _ := requests.Get(v.Pic)
				b.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeGroup,
					ToUserUid:  g,
					Content: OPQBot.SendTypePicMsgByBase64Content{
						Content: OPQBot.MacroAt([]int64{userId}) + fmt.Sprintf("您订阅的UP主%s更新了\n%s\n%s", upName, v.Title, v.Description),
						Base64:  base64.StdEncoding.EncodeToString(res.Content()),
						Flash:   false,
					},
				})
			}
		}
		for _, v := range fanju {
			title, gs ,userId:= bi.GetFanjuGroupsByMid(v.Result.Media.MediaID)
			for _, g := range gs {
				if v1, ok := Config.CoreConfig.GroupConfig[g]; ok {
					if !v1.Bili {
						break
					}
				}
				res, _ := requests.Get(v.Result.Media.Cover)
				b.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeGroup,
					ToUserUid:  g,
					Content: OPQBot.SendTypePicMsgByBase64Content{
						Content: OPQBot.MacroAt([]int64{userId}) + fmt.Sprintf("您订阅的番剧%s更新了\n%s", title, v.Result.Media.NewEp.IndexShow),
						Base64:  base64.StdEncoding.EncodeToString(res.Content()),
						Flash:   false,
					},
				})
			}
		}
	})
	c.AddJob(-1, "Yiqing" ,"* * 8,18 * * ? " , func() {
		client := &http.Client{}
		baseUrl := "https://m.sm.cn/api/rest?method=Huoshenshan.local"
		req ,err := http.NewRequest("GET",baseUrl,nil)
		req.Header.Add("User-Agent","Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1")
		req.Header.Add("referer" , "https://broccoli.uc.cn/" )
		if(err !=nil){
			panic(err)
		}
		response, _ := client.Do(req)
		defer response.Body.Close()
		s,err:=ioutil.ReadAll(response.Body)
		var res yiqing.YiqingRes
		json.Unmarshal(s, &res)
		ups := fmt.Sprintf("疫情报告")
		ups += fmt.Sprintf("%s-%s\n全国单日报告%s\n" ,res.Title ,res.Time,res.MainReport.Report)
		ups += fmt.Sprintf("[表情190][表情190][表情190]信息总览[表情190][表情190][表情190]\n")
		ups += fmt.Sprintf("[表情145]全国累计确诊%s个昨日新增%s个\n",res.ContryData.SureCnt,res.ContryData.YstCureCnt)
		ups += fmt.Sprintf("[表情145]全国现存确诊%s个昨日新增%s个\n",res.ContryData.RestSureCnt,res.ContryData.RestSureCntIncr)
		ups += fmt.Sprintf("[表情145]累计输入确诊%s个\n",res.ContryData.InputCnt)
		ups += fmt.Sprintf("[表情145]全国累计治愈%s个昨日新增%s个\n",res.ContryData.CureCnt,res.ContryData.YstCureCnt)
		ups += fmt.Sprintf("[表情66][表情66][表情66]疫情当下，请注意保护安全")
		b.SendGroupTextMsg(-1, fmt.Sprintf(ups))
		fmt.Println(ups)
	})
	err = b.AddEvent(OPQBot.EventNameOnFriendMessage, func(qq int64, packet *OPQBot.FriendMsgPack) {
		s := b.Session.SessionStart(packet.FromUin)
		if v, err := s.GetString("github"); err == nil {
			groupidI, err := s.Get("github_groupId")
			if err != nil {
				b.SendFriendTextMsg(packet.FromUin, err.Error())
				return
			}
			groupId, ok := groupidI.(int64)
			if !ok {
				b.SendFriendTextMsg(packet.FromUin, "内部错误")
				return
			}
			err = g.AddRepo(v, packet.Content, groupId)
			if err != nil {
				b.SendFriendTextMsg(packet.FromUin, err.Error())
				s.Delete("github")
				s.Delete("github_groupId")
				return
			}
			b.SendFriendTextMsg(packet.FromUin, "成功!")
			s.Delete("github")
			s.Delete("github_groupId")
		}
	})
	if err != nil {
		log.Println(err)
	}
	// 黑名单优先级高于白名单
	err = b.AddEvent(OPQBot.EventNameOnGroupMessage, BlackGroupList, WhiteGroupList, func(botQQ int64, packet *OPQBot.GroupMsgPack) {
		if packet.FromUserID == botQQ {
			return
		}
		Config.Lock.RLock()

		var c Config.GroupConfig
		if v, ok := Config.CoreConfig.GroupConfig[packet.FromGroupID]; ok {
			c = v
		} else {
			c = Config.CoreConfig.DefaultGroupConfig
		}
		Config.Lock.RUnlock()
		if !c.Enable {
			return
		}
		if m, err := regexp.MatchString(c.MenuKeyWord, packet.Content); err != nil {
			log.Println(err)
			return
		} else if m {
			b.Send(OPQBot.SendMsgPack{
				SendToType:   OPQBot.SendToTypeGroup,
				ToUserUid:    packet.FromGroupID,
				Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + c.Menu},
				CallbackFunc: nil,
			})
			return
		}
		//if m, err := regexp.MatchString(c.ShutUpWord, packet.Content); err != nil {
		//	log.Println(err)
		//	return
		//} else if m {
		//	err := b.ReCallMsg(packet.FromGroupID, packet.MsgRandom, packet.MsgSeq)
		//	if err != nil {
		//		log.Println(err)
		//	}
		//	err = b.SetForbidden(1, c.ShutUpTime, packet.FromGroupID, packet.FromUserID)
		//	if err != nil {
		//		log.Println(err)
		//	}
		//	return
		//}
		if v, _ := regexp.MatchString(`[0-9]{6}`, packet.Content); v {
			VerifyLock.Lock()
			if v1, ok := VerifyNum[strconv.FormatInt(packet.FromUserID, 10)+"|"+strconv.FormatInt(packet.FromGroupID, 10)]; ok {
				if v1.Code == packet.Content {
					v1.Status = true
					b.Send(OPQBot.SendMsgPack{
						SendToType: OPQBot.SendToTypeGroup,
						ToUserUid:  packet.FromGroupID,
						Content: OPQBot.SendTypeTextMsgContent{
							Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "验证成功",
						},
					})
				}
			}
			VerifyLock.Unlock()
		}
		if packet.Content == "刷新验证码" {
			VerifyLock.Lock()
			if v1, ok := VerifyNum[strconv.FormatInt(packet.FromUserID, 10)+"|"+strconv.FormatInt(packet.FromGroupID, 10)]; ok {
				picB, n, err := draw.Draw6Number()
				if err != nil {
					log.Println(err)
					return
				}
				v1.Code = n
				VerifyNum[strconv.FormatInt(packet.FromUserID, 10)+"|"+strconv.FormatInt(packet.FromGroupID, 10)] = v1
				b.Send(OPQBot.SendMsgPack{
					SendToType: OPQBot.SendToTypeGroup,
					ToUserUid:  packet.FromGroupID,
					Content: OPQBot.SendTypePicMsgByBase64Content{
						Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "请在5分钟内输入上方图片验证码！否则会被移出群,若看不清楚可以输入 刷新验证码\n" + OPQBot.MacroId(),
						Base64:  base64.StdEncoding.EncodeToString(picB),
						Flash:   false,
					},
				})
			}
			VerifyLock.Unlock()
			return
		}
		if packet.Content == "签到" {

			if !c.SignIn {
				b.Send(OPQBot.SendMsgPack{
					SendToType:   OPQBot.SendToTypeGroup,
					ToUserUid:    packet.FromGroupID,
					Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "本群签到功能未开启!"},
					CallbackFunc: nil,
				})
				return
			}
			Config.Lock.Lock()
			if v, ok := Config.CoreConfig.UserData[packet.FromUserID]; ok {
				if v.LastSignDay == time.Now().Day() {
					b.Send(OPQBot.SendMsgPack{
						SendToType:   OPQBot.SendToTypeGroup,
						ToUserUid:    packet.FromGroupID,
						Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "今日已经签到过了,明日再来"},
						CallbackFunc: nil,
					})
				} else {
					v.Count += 1
					v.LastSignDay = time.Now().Day()
					Config.CoreConfig.UserData[packet.FromUserID] = v
					err := Config.Save()
					if err != nil {
						log.Println(err)
					}
					b.Send(OPQBot.SendMsgPack{
						SendToType:   OPQBot.SendToTypeGroup,
						ToUserUid:    packet.FromGroupID,
						Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "签到成功"},
						CallbackFunc: nil,
					})
				}
			} else {
				v.Count = 1
				v.LastSignDay = time.Now().Day()
				Config.CoreConfig.UserData[packet.FromUserID] = v
				err := Config.Save()
				if err != nil {
					log.Println(err)
				}
				b.Send(OPQBot.SendMsgPack{
					SendToType:   OPQBot.SendToTypeGroup,
					ToUserUid:    packet.FromGroupID,
					Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "签到成功"},
					CallbackFunc: nil,
				})
			}
			Config.Lock.Unlock()
		}
		if packet.Content == "赞我" {
			if !c.Zan {
				b.Send(OPQBot.SendMsgPack{
					SendToType:   OPQBot.SendToTypeGroup,
					ToUserUid:    packet.FromGroupID,
					Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "本群名片赞功能未开启!"},
					CallbackFunc: nil,
				})
				return
			}
			Config.Lock.Lock()
			if v, ok := Config.CoreConfig.UserData[packet.FromUserID]; ok {
				if v.LastZanDay == time.Now().Day() {
					b.Send(OPQBot.SendMsgPack{
						SendToType:   OPQBot.SendToTypeGroup,
						ToUserUid:    packet.FromGroupID,
						Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "今日已经赞过了,明日再来"},
						CallbackFunc: nil,
					})
				} else {
					v.LastZanDay = time.Now().Day()
					Config.CoreConfig.UserData[packet.FromUserID] = v
					err := Config.Save()
					if err != nil {
						log.Println(err)
					}
					b.Send(OPQBot.SendMsgPack{
						SendToType:   OPQBot.SendToTypeGroup,
						ToUserUid:    packet.FromGroupID,
						Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "正在赞请稍后"},
						CallbackFunc: nil,
					})
				}
			} else {
				v.LastZanDay = time.Now().Day()
				Config.CoreConfig.UserData[packet.FromUserID] = v
				err := Config.Save()
				if err != nil {
					log.Println(err)
				}
				b.Send(OPQBot.SendMsgPack{
					SendToType:   OPQBot.SendToTypeGroup,
					ToUserUid:    packet.FromGroupID,
					Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "正在赞请稍后"},
					CallbackFunc: nil,
				})
			}
			Config.Lock.Unlock()
		}
		if packet.Content == "积分" {
			Config.Lock.RLock()
			if v, ok := Config.CoreConfig.UserData[packet.FromUserID]; ok {
				b.Send(OPQBot.SendMsgPack{
					SendToType:   OPQBot.SendToTypeGroup,
					ToUserUid:    packet.FromGroupID,
					Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "你的积分为" + strconv.Itoa(v.Count)},
					CallbackFunc: nil,
				})
			} else {
				b.Send(OPQBot.SendMsgPack{
					SendToType:   OPQBot.SendToTypeGroup,
					ToUserUid:    packet.FromGroupID,
					Content:      OPQBot.SendTypeTextMsgContent{Content: OPQBot.MacroAt([]int64{packet.FromUserID}) + "你的积分为0"},
					CallbackFunc: nil,
				})
			}
			Config.Lock.RUnlock()
		}
		cm := strings.Split(packet.Content, " ")
		s := b.Session.SessionStart(packet.FromUserID)
		if packet.Content == "退出订阅" {
			err := s.Delete("biliUps")
			if err != nil {
				log.Println(err)
			}
			b.SendGroupTextMsg(packet.FromGroupID, "已经退出订阅")
			return
		}
		if v, err := s.Get("biliUps"); err == nil {
			id, err := strconv.Atoi(packet.Content)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, "序号错误, 输入“退出订阅”退出")
				return
			}
			if v1, ok := v.(map[int]int64); ok {
				if v2, ok := v1[id]; ok {
					u, err := bi.SubscribeUpByMid(packet.FromGroupID, v2 ,packet.FromUserID)
					if err != nil {
						b.SendGroupTextMsg(packet.FromGroupID, err.Error())
						err = s.Delete("biliUps")
						if err != nil {
							log.Println(err)
						}
						return
					}
					r, _ := requests.Get(u.Data.Card.Face)
					b.SendGroupPicMsg(packet.FromGroupID, "成功订阅UP主"+u.Data.Card.Name, r.Content())
					err = s.Delete("biliUps")
					if err != nil {
						log.Println(err)
					}
					return
				} else {
					b.SendGroupTextMsg(packet.FromGroupID, "序号不存在")
					return
				}

			} else {
				b.SendGroupTextMsg(packet.FromGroupID, "内部错误")
				err := s.Delete("biliUps")
				if err != nil {
					log.Println(err)
				}
				return
			}
		}
		if packet.Content == "本群Github" {
			githubs := "本群订阅Github仓库\n"
			list := g.GetGroupSubList(packet.FromGroupID)
			if len(list) == 0 {
				b.SendGroupTextMsg(packet.FromGroupID, "本群没有订阅Github仓库")
				return
			}
			for k, _ := range list {
				githubs += fmt.Sprintf("%s \n", k)
			}
			b.SendGroupTextMsg(packet.FromGroupID, githubs)
		}
		if len(cm) == 2 && cm[0] == "取消订阅Github" {
			err := g.DelRepo(cm[1], packet.FromGroupID)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, err.Error())
				return
			}
			b.SendGroupTextMsg(packet.FromGroupID, "取消订阅成功!")

		}
		if len(cm) == 2 && cm[0] == "订阅Github" {
			b.SendGroupTextMsg(packet.FromGroupID, "请私聊我发送该仓库的Webhook Secret!")
			err := s.Set("github", cm[1])
			if err != nil {
				log.Println(err)
				return
			}
			err = s.Set("github_groupId", packet.FromGroupID)
			if err != nil {
				log.Println(err)
				return
			}

		}
		if len(cm) == 2 && cm[0] == "订阅up" {
			if !c.Bili {
				return
			}
			mid, err := strconv.ParseInt(cm[1], 10, 64)
			if err != nil {
				result, err := bi.SearchUp(cm[1])
				//u, err := bi.SubscribeUpByKeyword(packet.FromGroupID, cm[1])

				if err != nil {
					b.SendGroupTextMsg(packet.FromGroupID, err.Error())
					return
				}
				var (
					resultStr []string
					r         = map[int]int64{}
				)
				i := 0
				for _, v := range result.Data.Result {
					if v.IsUpuser == 1 {
						resultStr = append(resultStr, fmt.Sprintf("[%d] %s(lv.%d) 粉丝数:%d", i+1, v.Uname, v.Level, v.Fans))
						r[i+1] = v.Mid
						i++
						if len(r) >= 6 {
							break
						}
					}
				}
				if len(r) == 0 {
					b.SendGroupTextMsg(packet.FromGroupID, "没有找到UP哟~")
					return
				}
				err = s.Set("biliUps", r)
				if err != nil {
					b.SendGroupTextMsg(packet.FromGroupID, err.Error())
					return
				}
				b.SendGroupTextMsg(packet.FromGroupID, fmt.Sprintf("====输入序号选择UP====\n%s", strings.Join(resultStr, "\n")))
				return
			}
			u, err := bi.SubscribeUpByMid(packet.FromGroupID, mid , packet.FromUserID)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, err.Error())
				return
			}
			r, _ := requests.Get(u.Data.Card.Face)
			b.SendGroupPicMsg(packet.FromGroupID, "成功订阅UP主"+u.Data.Card.Name, r.Content())
		}
		if len(cm) == 2 && cm[0] == "取消订阅up" {
			if !c.Bili {
				return
			}
			mid, err := strconv.ParseInt(cm[1], 10, 64)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, "只能使用Mid取消订阅欧~")
				return
			}
			err = bi.UnSubscribeUp(packet.FromGroupID, mid)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, err.Error())
				return
			}
			b.SendGroupTextMsg(packet.FromGroupID, "成功取消订阅UP主")
		}
		if packet.Content == "本群up" {
			if !c.Bili {
				return
			}
			ups := "本群订阅UPs\n"

			if len(c.BiliUps) == 0 {
				b.SendGroupTextMsg(packet.FromGroupID, "本群没有订阅UP主")
				return
			}
			for mid, v1  := range c.BiliUps {
				ups += fmt.Sprintf("%d - %s -订阅者:%d\n", mid, v1.Name, v1.UserId)
			}
			b.SendGroupTextMsg(packet.FromGroupID, ups)

		}
		if len(cm) == 2 && cm[0] == "订阅番剧" {
			if !c.Bili {
				return
			}
			mid, err := strconv.ParseInt(cm[1], 10, 64)
			if err != nil {
				u, err := bi.SubscribeFanjuByKeyword(packet.FromGroupID, cm[1],packet.FromUserID)
				if err != nil {
					b.SendGroupTextMsg(packet.FromGroupID, err.Error())
					return
				}
				r, _ := requests.Get(u.Result.Media.Cover)
				b.SendGroupPicMsg(packet.FromGroupID, "成功订阅番剧"+u.Result.Media.Title, r.Content())
				return
			}
			u, err := bi.SubscribeFanjuByMid(packet.FromGroupID, mid,packet.FromUserID)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, err.Error())
				return
			}
			r, _ := requests.Get(u.Result.Media.Cover)
			b.SendGroupPicMsg(packet.FromGroupID, "成功订阅番剧"+u.Result.Media.Title, r.Content())
		}
		if len(cm) == 2 && cm[0] == "取消订阅番剧" {
			if !c.Bili {
				return
			}
			mid, err := strconv.ParseInt(cm[1], 10, 64)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, "只能使用Mid取消订阅欧~")
				return
			}
			err = bi.UnSubscribeFanju(packet.FromGroupID, mid)
			if err != nil {
				b.SendGroupTextMsg(packet.FromGroupID, err.Error())
				return
			}
			b.SendGroupTextMsg(packet.FromGroupID, "成功取消订阅番剧")
		}
		if packet.Content == "本群番剧" {
			if !c.Bili {
				return
			}
			ups := "本群订阅番剧\n"

			if len(c.Fanjus) == 0 {
				b.SendGroupTextMsg(packet.FromGroupID, "本群没有订阅番剧")
				return
			}
			for mid, v1 := range c.Fanjus {
				ups += fmt.Sprintf("%d - %s-订阅用户为：%d \n", mid, v1.Title,v1.UserId)
			}
			b.SendGroupTextMsg(packet.FromGroupID, ups)
		}
		if len(cm) == 2 && cm[0] == "梗查询"{
			b.SendGroupTextMsg(packet.FromGroupID, fmt.Sprintf("正在查询梗%s",cm[1]))
			client := &http.Client{}
			baseUrl := "https://api.jikipedia.com/go/search_entities"
			postData := make(map[string]interface{})
			postData["phrase"] = cm[1]
			postData["page"] = 1
			bytesData, err := json.Marshal(postData)
			if err != nil {
				fmt.Println(err.Error() )
				return
			}
			reader := bytes.NewReader(bytesData)
			req ,err := http.NewRequest("POST",baseUrl,reader)
			req.Header.Add("User-Agent","Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1")
			req.Header.Add("referer" , "https://broccoli.uc.cn/" )
			req.Header.Set("Content-Type", "application/json;charset=UTF-8")
			if(err !=nil){
				panic(err)
			}
			response, _ := client.Do(req)
			defer response.Body.Close()
			s,err:=ioutil.ReadAll(response.Body)
			var res GengChaxun.GenChaxunRes
			json.Unmarshal(s, &res)
			var content string
			for i , a := range res.Data {
				if i == 1{
					for j ,b := range a.Definitions{
						if j == 0 {
							content = b.Plaintext
						}
					}
				}
			}
			b.SendGroupTextMsg(packet.FromGroupID, fmt.Sprintf( "%s",content))
		}
		if packet.Content == "疫情信息"{
			b.SendGroupTextMsg(packet.FromGroupID, "正在查找信息")
			client := &http.Client{}
			baseUrl := "https://m.sm.cn/api/rest?method=Huoshenshan.local"
			req ,err := http.NewRequest("GET",baseUrl,nil)
			req.Header.Add("User-Agent","Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1")
			req.Header.Add("referer" , "https://broccoli.uc.cn/" )
			if(err !=nil){
				panic(err)
			}
			response, _ := client.Do(req)
			defer response.Body.Close()
			s,err:=ioutil.ReadAll(response.Body)
			var res yiqing.YiqingRes
			json.Unmarshal(s, &res)
			ups := fmt.Sprintf("%s-%s\n全国单日报告%s\n" ,res.Title ,res.Time,res.MainReport.Report)
			ups += fmt.Sprintf("[表情190][表情190][表情190]信息总览[表情190][表情190][表情190]\n")
			ups += fmt.Sprintf("[表情145]全国累计确诊%s个昨日新增%s个\n",res.ContryData.SureCnt,res.ContryData.YstCureCnt)
			ups += fmt.Sprintf("[表情145]全国现存确诊%s个昨日新增%s个\n",res.ContryData.RestSureCnt,res.ContryData.RestSureCntIncr)
			ups += fmt.Sprintf("[表情145]累计输入确诊%s个\n",res.ContryData.InputCnt)
			ups += fmt.Sprintf("[表情145]全国累计治愈%s个昨日新增%s个\n",res.ContryData.CureCnt,res.ContryData.YstCureCnt)
			ups += fmt.Sprintf("[表情66][表情66][表情66]疫情当下，请注意保护安全")
			b.SendGroupTextMsg(packet.FromGroupID, fmt.Sprintf(ups))
			fmt.Println(ups)
		}
	})
	if err != nil {
		log.Println(err)
	}
	err = b.AddEvent(OPQBot.EventNameOnGroupJoin, func(botQQ int64, packet *OPQBot.GroupJoinPack) {
		Config.Lock.RLock()
		var c Config.GroupConfig
		if v, ok := Config.CoreConfig.GroupConfig[packet.EventMsg.FromUin]; ok {
			c = v
		} else {
			c = Config.CoreConfig.DefaultGroupConfig
		}
		Config.Lock.RUnlock()
		if !c.Enable {
			return
		}
		switch c.JoinVerifyType {
		case 1: // 图片验证码
			picB, n, err := draw.Draw6Number()
			if err != nil {
				log.Println(err)
				return
			}
			b.Send(OPQBot.SendMsgPack{
				SendToType: OPQBot.SendToTypeGroup,
				ToUserUid:  packet.EventMsg.FromUin,
				Content: OPQBot.SendTypePicMsgByBase64Content{
					Content: OPQBot.MacroAt([]int64{packet.EventData.UserID}) + "请在5分钟内输入上方图片验证码！否则会被移出群,若看不清楚可以输入 刷新验证码\n" + OPQBot.MacroId(),
					Base64:  base64.StdEncoding.EncodeToString(picB),
					Flash:   false,
				},
				CallbackFunc: func(Code int, Info string, record OPQBot.MyRecord) {
					if record.MsgSeq == 0 {
						log.Println("验证码信息没有发送成功！")
					} else {
						VerifyLock.Lock()
						VerifyNum[strconv.FormatInt(packet.EventData.UserID, 10)+"|"+strconv.FormatInt(packet.EventMsg.FromUin, 10)] = &struct {
							Status bool
							Code   string
						}{Status: false, Code: n}
						VerifyLock.Unlock()
						time.Sleep(time.Duration(c.JoinVerifyTime) * time.Second)
						VerifyLock.Lock()
						log.Println(VerifyNum)
						if v, ok := VerifyNum[strconv.FormatInt(packet.EventData.UserID, 10)+"|"+strconv.FormatInt(packet.EventMsg.FromUin, 10)]; ok {
							if !v.Status {
								b.Send(OPQBot.SendMsgPack{
									SendToType: OPQBot.SendToTypeGroup,
									ToUserUid:  packet.EventMsg.FromUin,
									Content: OPQBot.SendTypeTextMsgContent{
										Content: OPQBot.MacroAt([]int64{packet.EventData.UserID}) + "验证超时,再见!",
									},
									CallbackFunc: func(Code int, Info string, record OPQBot.MyRecord) {
										b.KickGroupMember(packet.EventMsg.FromUin, packet.EventData.UserID)
									},
								})
							} else {
								delete(VerifyNum, strconv.FormatInt(packet.EventData.UserID, 10)+"|"+strconv.FormatInt(packet.EventMsg.FromUin, 10))
							}
						}
						VerifyLock.Unlock()
					}
				},
			})

		default:
		}
		if c.Welcome != "" {
			b.SendGroupTextMsg(packet.EventMsg.FromUin, c.Welcome)
		}
		if c.JoinAutoShutUpTime != 0 {
			b.SetForbidden(1, c.JoinAutoShutUpTime, packet.EventMsg.FromUin, packet.EventData.UserID)
		}
	})
	err = b.AddEvent(OPQBot.EventNameOnConnected, func() {
		log.Println("连接服务器成功")
	})
	if err != nil {
		log.Println(err)
	}
	err = b.Start()
	if err != nil {
		log.Println(err)
	}
	if Config.CoreConfig.OPQWebConfig.Enable {
		log.Println("启动Web 😊")
		go func() {

			Config.Lock.Lock()
			sess = sessions.New(sessions.Config{Cookie: "OPQWebSession"})
			if Config.CoreConfig.OPQWebConfig.CSRF == "" {
				Config.CoreConfig.OPQWebConfig.CSRF = utils.RandomString(32)
				err := Config.Save()
				if err != nil {
					log.Println(err)
				}
			}
			fads, _ := fs.Sub(staticFs, "Web/dist/spa")
			if Config.CoreConfig.ReverseProxy != "" {
				// target, err := url.Parse(Config.CoreConfig.ReverseProxy)
				if err != nil {
					panic(err)
				}
				app.Get("{root:path}", func(ctx iris.Context) {
					director := func(r *http.Request) {
						r.Host = Config.CoreConfig.ReverseProxy
						r.URL, _ = url.Parse(r.Host + "/" + ctx.Path())
					}
					p := &httputil.ReverseProxy{Director: director}
					p.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
				})
			} else {
				app.HandleDir("/", http.FS(fads))
			}

			// app.HandleDir("/", iris.Dir("./Web/dist/spa"))
			Config.Lock.Unlock()

			app.Use(beforeCsrf)
			app.Use(sess.Handler())
			app.WrapRouter(func(w http.ResponseWriter, r *http.Request, router http.HandlerFunc) {
				w.Header().Add("Access-Control-Allow-Origin", r.Header.Get("Origin"))
				w.Header().Add("Access-Control-Allow-Credentials", "true")
				path := r.URL.Path
				if r.Method == "OPTIONS" {
					w.Header().Add("Access-Control-Allow-Headers", "content-type")
					w.WriteHeader(200)
					return
				}
				if len(path) < 4 {
					if !pathIsFile(path) {
						r.URL.Path = "/"
					}
				} else {
					if r.URL.Path[0:4] != "/api" && r.URL.Path[0:4] != "/git" {
						if !pathIsFile(path) {
							r.URL.Path = "/"
						}
					}
				}
				// log.Println(r.URL.Path)
				router.ServeHTTP(w, r)
			})
			app.Get("/api/csrf", func(ctx iris.Context) {
				s := sess.Start(ctx)
				salt := int(time.Now().Unix())
				keyTmp := methods.Md5V(strconv.Itoa(salt + rand.Intn(100)))
				s.Set("OPQWebCSRF", keyTmp)
				ctx.SetCookieKV("OPQWebCSRF", keyTmp, iris.CookieHTTPOnly(false))
				_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: s.Get("username")})
			})
			app.Get("/api/status", func(ctx iris.Context) {
				s := sess.Start(ctx)
				salt := int(time.Now().Unix())
				keyTmp := methods.Md5V(strconv.Itoa(salt + rand.Intn(100)))
				s.Set("OPQWebCSRF", keyTmp)
				ctx.SetCookieKV("OPQWebCSRF", keyTmp, iris.CookieHTTPOnly(false))
				if s.GetBooleanDefault("auth", false) {
					_, _ = ctx.JSON(WebResult{Code: 1, Info: "已登录!", Data: s.Get("username")})
					return
				} else {
					_, _ = ctx.JSON(WebResult{Code: 0, Info: "未登录!", Data: nil})
					return
				}
			})
			app.Post("/api/login", func(ctx iris.Context) {
				username := ctx.FormValue("username")
				password := ctx.FormValue("password")
				Config.Lock.RLock()
				defer Config.Lock.RUnlock()
				if username == Config.CoreConfig.OPQWebConfig.Username && password == methods.Md5V(Config.CoreConfig.OPQWebConfig.Password) {
					s := sess.Start(ctx)
					s.Set("auth", true)
					_, _ = ctx.JSON(WebResult{Code: 1, Info: "登录成功", Data: nil})
					return
				} else {
					_, _ = ctx.JSON(WebResult{Code: 0, Info: "用户名密码错误!", Data: nil})
					return
				}

			})
			// job周期任务读取
			Config.Lock.RLock()
			for k, v := range Config.CoreConfig.GroupConfig {
				for k1, v2 := range v.Job {
					switch v2.Type {
					case 1:
						err = c.AddJob(k, k1, v2.Cron, func() {
							log.Print("执行任务" + k1)
							if b.Announce(v2.Title, v2.Content, 0, 10, k) != nil {
								log.Print(err)
							}
						})
						if err != nil {
							log.Print("添加任务" + k1 + "出现错误" + err.Error())
						}
					case 2:
						err = c.AddJob(k, k1, v2.Cron, func() {
							log.Print("执行任务" + k1)
							if b.SetForbidden(0, 1, k, 0) != nil {
								log.Print(err)
							}
						})
						if err != nil {
							log.Print("添加任务" + k1 + "出现错误" + err.Error())
						}
					case 3:
						err = c.AddJob(k, k1, v2.Cron, func() {
							log.Print("执行任务" + k1)
							if b.SetForbidden(0, 0, k, 0) != nil {
								log.Print(err)
							}
						})
						if err != nil {
							log.Print("添加任务" + k1 + "出现错误" + err.Error())
						}
					case 4:
						err = c.AddJob(k, k1, v2.Cron, func() {
							log.Print("执行任务" + k1)
							b.Send(OPQBot.SendMsgPack{
								SendToType: OPQBot.SendToTypeGroup,
								ToUserUid:  k,
								Content: OPQBot.SendTypeTextMsgContent{
									Content: v2.Content,
								},
							})
						})
						if err != nil {
							log.Print("添加任务" + k1 + "出现错误" + err.Error())
						}
					}
				}
			}
			Config.Lock.RUnlock()
			needAuth := app.Party("/api/admin", requireAuth)
			{
				rJob := needAuth.Party("/job")
				{
					rJob.Post("/add", func(ctx iris.Context) {
						ids := ctx.FormValue("id")
						id, err := strconv.ParseInt(ids, 10, 64)
						if err != nil {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
							return
						}
						if id == -1 {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: "默认群禁止添加周期任务", Data: nil})
							return
						}
						span := ctx.FormValue("span")
						jobName := ctx.FormValue("jobName")
						if jobName == "" {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: "jobName为空", Data: nil})
							return
						}
						cronType, _ := strconv.Atoi(ctx.FormValue("type"))
						switch cronType {
						// 公告
						case 1:
							title := ctx.FormValue("title")
							content := ctx.FormValue("content")
							err = c.AddJob(id, jobName, span, func() {
								log.Print("执行任务" + jobName)
								if b.Announce(title, content, 0, 10, id) != nil {
									log.Print(err)
								}
							})
							if err != nil {
								_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
								return
							} else {
								job := Config.Job{Type: cronType, Cron: span, Title: title, Content: content}
								Config.Lock.Lock()
								defer Config.Lock.Unlock()
								if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								} else {
									v = Config.CoreConfig.DefaultGroupConfig
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								}
								Config.Save()
								_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
								return
							}
						// 全局禁言
						case 2:
							err = c.AddJob(id, jobName, span, func() {
								log.Print("执行任务" + jobName)
								if b.SetForbidden(0, 1, id, 0) != nil {
									log.Print(err)
								}
							})
							if err != nil {
								_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
								return
							} else {
								job := Config.Job{Type: cronType, Cron: span}
								Config.Lock.Lock()
								defer Config.Lock.Unlock()
								if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								} else {
									v = Config.CoreConfig.DefaultGroupConfig
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								}
								Config.Save()
								_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
								return
							}
						// 解除全局禁言
						case 3:
							err = c.AddJob(id, jobName, span, func() {
								log.Print("执行任务" + jobName)
								if b.SetForbidden(0, 0, id, 0) != nil {
									log.Print(err)
								}
							})
							if err != nil {
								_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
								return
							} else {
								job := Config.Job{Type: cronType, Cron: span}
								Config.Lock.Lock()
								defer Config.Lock.Unlock()
								if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								} else {
									v = Config.CoreConfig.DefaultGroupConfig
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								}
								Config.Save()
								_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
								return
							}
						case 4:
							// title := ctx.FormValue("title")
							content := ctx.FormValue("content")
							err = c.AddJob(id, jobName, span, func() {
								log.Print("执行任务" + jobName)
								b.Send(OPQBot.SendMsgPack{
									SendToType: OPQBot.SendToTypeGroup,
									ToUserUid:  id,
									Content: OPQBot.SendTypeTextMsgContent{
										Content: content,
									},
								})
							})
							if err != nil {
								_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
								return
							} else {
								job := Config.Job{Type: cronType, Cron: span, Content: content}
								Config.Lock.Lock()
								defer Config.Lock.Unlock()
								if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								} else {
									v = Config.CoreConfig.DefaultGroupConfig
									v.Job[jobName] = job
									Config.CoreConfig.GroupConfig[id] = v
								}
								Config.Save()
								_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
								return
							}
						default:
							_, _ = ctx.JSON(WebResult{Code: 0, Info: "类型不存在", Data: nil})
							return
						}
					})
					rJob.Post("/del", func(ctx iris.Context) {
						ids := ctx.FormValue("id")
						id, err := strconv.ParseInt(ids, 10, 64)
						if err != nil {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
							return
						}
						if id == -1 {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: "默认群禁止删除周期任务", Data: nil})
							return
						}
						jobName := ctx.FormValue("jobName")
						if jobName == "" {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: "jobName为空", Data: nil})
							return
						}
						err = c.Remove(id, jobName)

						if err != nil {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
							return
						}
						Config.Lock.Lock()
						defer Config.Lock.Unlock()
						if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
							delete(v.Job, jobName)
							Config.CoreConfig.GroupConfig[id] = v
						} else {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: "Group在配置文件中不存在！", Data: nil})
							return
						}
						Config.Save()
						_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
					})
				}
				needAuth.Post("/getGroupMember", func(ctx iris.Context) {
					ids := ctx.FormValue("id")
					id, err := strconv.ParseInt(ids, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					if id == -1 {
						_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: []int{}})
						return
					}

					glist, err := b.GetGroupMemberList(id, 0)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					result := glist
					for {
						if glist.LastUin == 0 {
							break
						}
						glist, err = b.GetGroupMemberList(id, glist.LastUin)
						if err != nil {
							_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
							return
						}
						result.MemberList = append(result.MemberList, glist.MemberList...)
						result.Count += glist.Count
						result.LastUin = glist.LastUin
					}
					_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: result})
					return
				})
				needAuth.Post("/setGroupConfig", func(ctx iris.Context) {
					ids := ctx.FormValue("id")
					enable := ctx.FormValue("enable")
					id, err := strconv.ParseInt(ids, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					if enable != "" {
						Config.Lock.Lock()
						defer Config.Lock.Unlock()
						Enable := ctx.FormValue("enable") == "true"
						if id == -1 {
							Config.CoreConfig.DefaultGroupConfig.Enable = Enable
							_, _ = ctx.JSON(WebResult{
								Code: 1,
								Info: "默认配置保存成功!",
								Data: Config.CoreConfig.GroupConfig[id].Enable,
							})
							err := Config.Save()
							if err != nil {
								log.Println(err)
							}
							return
						}
						if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
							v.Enable = Enable
							Config.CoreConfig.GroupConfig[id] = v
						} else {
							v = Config.CoreConfig.DefaultGroupConfig
							v.Enable = Enable
							Config.CoreConfig.GroupConfig[id] = v
						}
						_, _ = ctx.JSON(WebResult{
							Code: 1,
							Info: "保存成功!",
							Data: Config.CoreConfig.GroupConfig[id].Enable,
						})
						err := Config.Save()
						if err != nil {
							log.Println(err)
						}
						return
					}
					menuData := ctx.FormValue("data[Menu]")
					menuKeyWordData := ctx.FormValue("data[MenuKeyWord]")
					Enable := ctx.FormValue("data[Enable]") == "true"
					ShutUpWord := ctx.FormValue("data[ShutUpWord]")
					Welcome := ctx.FormValue("data[Welcome]")
					AdminUin, _ := strconv.ParseInt(ctx.FormValue("data[AdminUin]"), 10, 64)
					JoinVerifyTime, _ := strconv.Atoi(ctx.FormValue("data[JoinVerifyTime]"))
					JoinAutoShutUpTime, _ := strconv.Atoi(ctx.FormValue("data[JoinAutoShutUpTime]"))
					ShutUpTime, _ := strconv.Atoi(ctx.FormValue("data[ShutUpTime]"))
					JoinVerifyType, _ := strconv.Atoi(ctx.FormValue("data[JoinVerifyType]"))
					Zan := ctx.FormValue("data[Zan]") == "true"
					Bili := ctx.FormValue("data[Bili]") == "true"
					SignIn := ctx.FormValue("data[SignIn]") == "true"
					Job := map[string]Config.Job{}
					for k, v := range ctx.FormValues() {
						//log.Println(k,strings.HasPrefix(k,"data[Job]["),strings.Split(strings.TrimPrefix(k,"data[Job]["),"]"))
						if strings.HasPrefix(k, "data[Job][") {
							if v1 := strings.Split(strings.TrimPrefix(k, "data[Job]["), "]"); len(v1) >= 2 && len(v) >= 1 {
								switch v1[1] {
								case "[Cron":
									v2, _ := Job[v1[0]]
									v2.Cron = v[0]
									Job[v1[0]] = v2
								case "[JobType":
									v2, _ := Job[v1[0]]
									v2.Type, _ = strconv.Atoi(v[0])
									Job[v1[0]] = v2
								case "[Content":
									v2, _ := Job[v1[0]]
									v2.Content = v[0]
									Job[v1[0]] = v2
								}

							}
						}
					}
					Config.Lock.Lock()
					defer Config.Lock.Unlock()

					if id == -1 {
						Config.CoreConfig.DefaultGroupConfig = Config.GroupConfig{BiliUps: map[int64]Config.Up{}, Bili: Bili, Job: Job, JoinVerifyType: JoinVerifyType, Welcome: Welcome, SignIn: SignIn, Zan: Zan, JoinVerifyTime: JoinVerifyTime, JoinAutoShutUpTime: JoinAutoShutUpTime, AdminUin: AdminUin, Menu: menuData, MenuKeyWord: menuKeyWordData, Enable: Enable, ShutUpWord: ShutUpWord, ShutUpTime: ShutUpTime}
						Config.Save()
						_, _ = ctx.JSON(WebResult{
							Code: 1,
							Info: "默认配置，保存成功!",
							Data: nil,
						})
						return
					}
					Config.CoreConfig.GroupConfig[id] = Config.GroupConfig{BiliUps: Config.CoreConfig.GroupConfig[id].BiliUps, Bili: Bili, Job: Job, JoinVerifyType: JoinVerifyType, Welcome: Welcome, SignIn: SignIn, Zan: Zan, JoinVerifyTime: JoinVerifyTime, JoinAutoShutUpTime: JoinAutoShutUpTime, AdminUin: AdminUin, Menu: menuData, MenuKeyWord: menuKeyWordData, Enable: Enable, ShutUpWord: ShutUpWord, ShutUpTime: ShutUpTime}
					Config.Save()
					_, _ = ctx.JSON(WebResult{
						Code: 1,
						Info: "保存成功!",
						Data: nil,
					})
					return
				})
				needAuth.Post("/groupStatus", func(ctx iris.Context) {
					ids := ctx.FormValue("id")
					id, err := strconv.ParseInt(ids, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					Config.Lock.RLock()
					defer Config.Lock.RUnlock()
					if id == -1 {
						_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: Config.CoreConfig.DefaultGroupConfig})
						return
					}
					if v, ok := Config.CoreConfig.GroupConfig[id]; ok {
						_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: v})
						return
					} else {
						_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: Config.CoreConfig.DefaultGroupConfig})
						return
					}
				})
				needAuth.Get("/groups", func(ctx iris.Context) {
					g, err := b.GetGroupList("")
					if err != nil {
						_, _ = ctx.JSON(WebResult{
							Code: 0,
							Info: err.Error(),
							Data: nil,
						})
						return
					}
					_, _ = ctx.JSON(WebResult{
						Code: 1,
						Info: "success",
						Data: g,
					})
				})
				needAuth.Post("/shutUp", func(ctx iris.Context) {
					ids := ctx.FormValue("id")
					uins := ctx.FormValue("uin")
					times := ctx.FormValue("time")
					id, err := strconv.ParseInt(ids, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					uin, err := strconv.ParseInt(uins, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					time1, err := strconv.Atoi(times)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					err = b.SetForbidden(1, time1, id, uin)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
					return
				})
				needAuth.Post("/kick", func(ctx iris.Context) {
					ids := ctx.FormValue("id")
					uins := ctx.FormValue("uin")
					id, err := strconv.ParseInt(ids, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					uin, err := strconv.ParseInt(uins, 10, 64)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					err = b.KickGroupMember(id, uin)
					if err != nil {
						_, _ = ctx.JSON(WebResult{Code: 0, Info: err.Error(), Data: nil})
						return
					}
					_, _ = ctx.JSON(WebResult{Code: 1, Info: "success", Data: nil})
					return
				})
				needAuth.Get("/logout", func(ctx iris.Context) {
					s := sess.Start(ctx)
					s.Set("auth", false)
					s.Clear()
					_, _ = ctx.JSON(WebResult{
						Code: 1,
						Info: "Success",
						Data: nil,
					})
				})
			}
			app.Logger().Prefix = "[Web]"
			err := app.Run(iris.Addr(Config.CoreConfig.OPQWebConfig.Host+":"+strconv.Itoa(Config.CoreConfig.OPQWebConfig.Port)), iris.WithoutStartupLog)
			if err != nil {
				log.Println(err)
				return
			}
		}()
	}
	b.Wait()
}

func WhiteGroupList(botQQ int64, packet *OPQBot.GroupMsgPack) {
	if len(Config.CoreConfig.WhiteGroupList) == 0 {
		packet.Next(botQQ, packet)
		return
	}
	isWhite := false
	for _, v := range Config.CoreConfig.WhiteGroupList {
		if v == packet.FromGroupID {
			isWhite = true
			break
		}
	}
	if isWhite {
		packet.Next(botQQ, &packet)
	}
}
func BlackGroupList(botQQ int64, packet *OPQBot.GroupMsgPack) {
	if len(Config.CoreConfig.BlackGroupList) == 0 {
		packet.Next(botQQ, packet)
		return
	}
	isBlack := false
	for _, v := range Config.CoreConfig.WhiteGroupList {
		if v == packet.FromGroupID {
			isBlack = true
			break
		}
	}
	if !isBlack {
		packet.Next(botQQ, packet)
	}
}
func requireAuth(ctx iris.Context) {
	s := sess.Start(ctx)
	if s.GetBooleanDefault("auth", false) {
		ctx.Next()
	} else {
		_, _ = ctx.JSON(WebResult{Code: 10010, Info: "未登录!", Data: nil})
		return
	}
}
func pathIsFile(path string) (isFile bool) {
	isFile = false
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			isFile = true
			break
		}
	}
	return
}
func beforeCsrf(ctx iris.Context) {
	s := sess.Start(ctx)
	//log.Println(s.Get("OPQWebCSRF"))
	if ctx.Method() == "POST" {
		if key := s.GetStringDefault("OPQWebCSRF", ""); key != "" && (ctx.GetHeader("csrfToken") == key || ctx.FormValue("csrfToken") == key) {
			ctx.Next()
		} else {
			// log.Println(key, "-", ctx.FormValue("csrfToken"))
			ctx.StatusCode(419)
			_, _ = ctx.JSON(WebResult{Code: 419, Info: "CSRF Error!", Data: nil})
			return
		}
	} else {
		ctx.Next()
	}
}
