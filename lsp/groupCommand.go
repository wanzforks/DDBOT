package lsp

import (
	"errors"
	"fmt"
	miraiBot "github.com/Logiase/MiraiGo-Template/bot"
	"github.com/Mrs4s/MiraiGo/message"
	"github.com/Sora233/Sora233-MiraiGo/concern"
	"github.com/Sora233/Sora233-MiraiGo/image_pool"
	"github.com/Sora233/Sora233-MiraiGo/image_pool/lolicon_pool"
	"github.com/Sora233/Sora233-MiraiGo/lsp/aliyun"
	"github.com/Sora233/Sora233-MiraiGo/lsp/bilibili"
	localdb "github.com/Sora233/Sora233-MiraiGo/lsp/buntdb"
	"github.com/Sora233/Sora233-MiraiGo/lsp/douyu"
	"github.com/Sora233/Sora233-MiraiGo/utils"
	"github.com/alecthomas/kong"
	"github.com/forestgiant/sliceutil"
	"github.com/tidwall/buntdb"
	"math/rand"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LspGroupCommand struct {
	bot *miraiBot.Bot
	msg *message.GroupMessage
	l   *Lsp

	args  []string
	debug bool
	exit  bool
}

func NewLspGroupCommand(bot *miraiBot.Bot, msg *message.GroupMessage, l *Lsp) *LspGroupCommand {
	return &LspGroupCommand{
		bot: bot,
		msg: msg,
		l:   l,
	}
}

func (lgc *LspGroupCommand) Exit(int) {
	lgc.exit = true
}

func (lgc *LspGroupCommand) Debug() {
	lgc.debug = true
}

func (lgc *LspGroupCommand) Execute() {
	defer func() {
		if err := recover(); err != nil {
			logger.WithField("stack", string(debug.Stack())).
				Errorf("panic recovered")
		}
	}()
	if lgc.debug {
		var ok bool
		if sliceutil.Contains([]int64{12532362}, lgc.msg.GroupCode) {
			ok = true
		}
		if sliceutil.Contains([]int64{382652405}, lgc.msg.Sender) {
			ok = true
		}
		if !ok {
			return
		}
	}
	if text, ok := lgc.msg.Elements[0].(*message.TextElement); ok {
		args := strings.Split(text.Content, " ")
		switch strings.TrimSpace(args[0]) {
		case "/lsp":
			lgc.LspCommand()
		case "/色图":
			lgc.SetuCommand(false)
		case "/黄图":
			if lgc.l.CheckGroupCommandPermission(lgc.msg.GroupCode, lgc.msg.Sender.Uin, "黄图") {
				lgc.SetuCommand(true)
			} else {
				lgc.textReply("权限不够")
			}
		case "/watch":
			lgc.WatchCommand(false)
		case "/unwatch":
			lgc.WatchCommand(true)
		case "/list":
			lgc.ListCommand()
		case "/签到":
			lgc.CheckinCommand()
		case "/roll":
			lgc.RollCommand()
		default:
		}
	} else {
		if lgc.msg.Sender.Uin != lgc.bot.Uin {
			lgc.ImageContent()
		}
	}
}

func (lgc *LspGroupCommand) LspCommand() {
	msg := lgc.msg
	groupCode := msg.GroupCode

	log := logger.WithField("GroupCode", groupCode)
	log.Infof("run lsp command")
	defer log.Info("lsp command end")

	var lspCmd struct{}
	lgc.parseArgs(&lspCmd, "lsp")
	if lgc.exit {
		return
	}
	lgc.textReply("LSP竟然是你")
	return
}

func (lgc *LspGroupCommand) SetuCommand(r18 bool) {
	msg := lgc.msg
	bot := lgc.bot
	groupCode := msg.GroupCode

	log := logger.WithField("GroupCode", groupCode)
	log.Info("run setu command")
	defer log.Info("setu command end")

	var setuCmd struct {
		Num int `arg:"" optional:"" help:"image number"`
	}
	var name string
	if r18 {
		name = "黄图"
	} else {
		name = "色图"
	}
	lgc.parseArgs(&setuCmd, name)
	if lgc.exit {
		return
	}

	num := setuCmd.Num

	if num <= 0 {
		num = 1
	}
	if num > 10 {
		num = 10
	}

	sendingMsg := message.NewSendingMessage()

	var options []image_pool.OptionFunc
	if r18 {
		options = append(options, lolicon_pool.R18Option(lolicon_pool.R18_ON))
	} else {
		options = append(options, lolicon_pool.R18Option(lolicon_pool.R18_OFF))
	}
	options = append(options, lolicon_pool.NumOption(num))
	imgs, err := lgc.l.GetImageFromPool(options...)
	if err != nil {
		log.Errorf("get from image pool failed %v", err)
		lgc.textReply("获取失败")
		return
	}
	if len(imgs) == 0 {
		log.Errorf("get empty image")
		lgc.textReply("获取失败")
		return
	}
	var imgsBytes = make([][]byte, len(imgs))
	var errs = make([]error, len(imgs))
	var groupImages = make([]*message.GroupImageElement, len(imgs))
	var wg sync.WaitGroup

	for index := range imgs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			imgsBytes[index], errs[index] = imgs[index].Content()
		}(index)
	}
	wg.Wait()

	for index := range imgs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			imgBytes, err := imgsBytes[index], errs[index]
			if err != nil {
				errs[index] = fmt.Errorf("get image bytes failed %v", err)
				return
			}
			resizedImage, err := utils.ImageNormSize(imgBytes)
			if err != nil {
				logger.Errorf("resize failed, use raw image")
				groupImages[index], errs[index] = bot.UploadGroupImage(groupCode, imgBytes)
			} else {
				groupImages[index], errs[index] = bot.UploadGroupImage(groupCode, resizedImage)
			}
		}(index)
	}
	wg.Wait()

	imgBatch := 2
	ok := false

	for i := 0; i < len(groupImages); i += imgBatch {
		last := i + imgBatch
		if last > len(groupImages) {
			last = len(groupImages)
		}
		groupPart := groupImages[i:last]

		for index, groupImage := range groupPart {
			if errs[i+index] != nil {
				continue
			}
			ok = true
			img := imgs[i+index]
			sendingMsg.Append(groupImage)
			if loliconImage, ok := img.(*lolicon_pool.Setu); ok {
				log.WithField("author", loliconImage.Author).
					WithField("r18", loliconImage.R18).
					WithField("pid", loliconImage.Pid).
					WithField("tags", loliconImage.Tags).
					WithField("title", loliconImage.Title).
					WithField("upload_url", groupImage.Url).
					Debug("debug image")
				sendingMsg.Append(message.NewText(fmt.Sprintf("标题：%v\n", loliconImage.Title)))
				sendingMsg.Append(message.NewText(fmt.Sprintf("作者：%v\n", loliconImage.Author)))
				sendingMsg.Append(message.NewText(fmt.Sprintf("PID：%v\n", loliconImage.Pid)))
				tagCount := len(loliconImage.Tags)
				if tagCount >= 2 {
					tagCount = 2
				}
				sendingMsg.Append(message.NewText(fmt.Sprintf("TAG：%v\n", strings.Join(loliconImage.Tags[:tagCount], " "))))
				sendingMsg.Append(message.NewText(fmt.Sprintf("R18：%v", loliconImage.R18)))
			}
		}
		lgc.reply(sendingMsg)
		sendingMsg = message.NewSendingMessage()
	}

	if !ok {
		lgc.textReply("获取失败")
	}
	return
}

func (lgc *LspGroupCommand) WatchCommand(remove bool) {
	var (
		msg       = lgc.msg
		groupCode = msg.GroupCode
		site      = bilibili.Site
		watchType = concern.BibiliLive
		err       error
	)

	log := logger.WithField("GroupCode", groupCode)
	log.Info("run watch command")
	defer log.Info("watch command end")

	var watchCmd struct {
		Site string `optional:"" short:"s" default:"bilibili" help:"bilibili / douyu"`
		Type string `optional:"" short:"t" default:"live" help:"news / live"`
		Id   int64  `arg:""`
	}
	var name string
	if remove {
		name = "unwatch"
	} else {
		name = "watch"
	}
	lgc.parseArgs(&watchCmd, name)
	if lgc.exit {
		return
	}

	site, watchType, err = lgc.parseRawSiteAndType(watchCmd.Site, watchCmd.Type)
	if err != nil {
		log.WithField("args", lgc.getArgs()).Errorf("parse raw concern failed %v", err)
		lgc.textReply(fmt.Sprintf("参数错误 - %v", err))
		return
	}
	log = log.WithField("site", site).WithField("type", watchType)

	id := watchCmd.Id

	switch site {
	case bilibili.Site:
		if remove {
			// unwatch
			if err := lgc.l.bilibiliConcern.Remove(groupCode, id, watchType); err != nil {
				lgc.textReply(fmt.Sprintf("unwatch失败 - %v", err))
			} else {
				log.WithField("mid", id).Debugf("unwatch success")
				lgc.textReply("unwatch成功")
			}
			return
		}
		// watch
		userInfo, err := lgc.l.bilibiliConcern.Add(groupCode, id, watchType)
		if err != nil {
			log.WithField("mid", id).Errorf("watch error %v", err)
			lgc.textReply(fmt.Sprintf("watch失败 - %v", err))
			return
		}
		log.WithField("mid", id).Debugf("watch success")
		lgc.textReply(fmt.Sprintf("watch成功 - Bilibili用户 %v", userInfo.Name))
	case douyu.Site:
		if remove {
			// unwatch
			if err := lgc.l.douyuConcern.Remove(groupCode, id, watchType); err != nil {
				lgc.textReply(fmt.Sprintf("unwatch失败 - %v", err))
			} else {
				log.WithField("mid", id).Debugf("unwatch success")
				lgc.textReply("unwatch成功")
			}
			return
		}
		// watch
		userInfo, err := lgc.l.douyuConcern.Add(groupCode, id, watchType)
		if err != nil {
			log.WithField("mid", id).Errorf("watch error %v", err)
			lgc.textReply(fmt.Sprintf("watch失败 - %v", err))
			break
		}
		log.WithField("mid", id).Debugf("watch success")
		lgc.textReply(fmt.Sprintf("watch成功 - 斗鱼用户 %v", userInfo.Nickname))
	default:
		log.WithField("site", site).Error("unsupported")
		lgc.textReply("未支持的网站")
	}
}

func (lgc *LspGroupCommand) ListCommand() {
	msg := lgc.msg
	groupCode := msg.GroupCode

	log := logger.WithField("GroupCode", groupCode)
	log.Info("run list living command")
	defer log.Info("list living command end")

	var listLivingCmd struct {
		Site string `optional:"" short:"s" default:"bilibili" help:"bilibili / douyu"`
		Type string `optional:"" short:"t" default:"live" help:"news / live"`
		All  bool   `optional:"" short:"a" default:"false" help:"show all"`
	}
	lgc.parseArgs(&listLivingCmd, "list")
	if lgc.exit {
		return
	}

	site, ctype, err := lgc.parseRawSiteAndType(listLivingCmd.Site, listLivingCmd.Type)
	if err != nil {
		log.WithField("args", lgc.getArgs()).Errorf("parse raw site failed %v", err)
		lgc.textReply(fmt.Sprintf("失败 - %v", err))
		return
	}
	log = log.WithField("site", site).WithField("type", ctype)

	all := listLivingCmd.All

	listMsg := message.NewSendingMessage()

	switch ctype {
	case concern.BibiliLive:
		listMsg.Append(message.NewText("当前直播：\n"))
		living, err := lgc.l.bilibiliConcern.ListLiving(groupCode, all)
		if err != nil {
			log.Debugf("list living failed %v", err)
			lgc.textReply(fmt.Sprintf("list living 失败 - %v", err))
			return
		}
		if living == nil {
			lgc.textReply("关注列表为空，可以使用/watch命令关注")
			return
		}
		for idx, liveInfo := range living {
			if idx != 0 {
				listMsg.Append(message.NewText("\n"))
			}
			notifyMsg := lgc.l.NotifyMessage(lgc.bot, liveInfo)
			for _, msg := range notifyMsg {
				listMsg.Append(msg)
			}
		}
		if len(listMsg.Elements) == 0 {
			listMsg.Append(message.NewText("无人直播"))
		}
	case concern.BilibiliNews:
		listMsg.Append(message.NewText("当前关注：\n"))
		news, err := lgc.l.bilibiliConcern.ListNews(groupCode, all)
		if err != nil {
			log.Debugf("list news failed %v", err)
			lgc.textReply(fmt.Sprintf("list news 失败 - %v", err))
			return
		}
		if news == nil {
			lgc.textReply("关注列表为空，可以使用/watch命令关注")
			return
		}
		for idx, newsInfo := range news {
			if idx != 0 {
				listMsg.Append(message.NewText("\n"))
			}
			listMsg.Append(message.NewText(newsInfo.Name))
		}
	case concern.DouyuLive:
		living, err := lgc.l.douyuConcern.ListLiving(groupCode, all)
		if err != nil {
			log.Debugf("list living failed %v", err)
			lgc.textReply(fmt.Sprintf("list living 失败 - %v", err))
			return
		}
		if living == nil {
			lgc.textReply("关注列表为空，可以使用/watch命令关注")
			return
		}
		for idx, liveInfo := range living {
			if idx != 0 {
				listMsg.Append(message.NewText("\n"))
			}
			notifyMsg := lgc.l.NotifyMessage(lgc.bot, liveInfo)
			for _, msg := range notifyMsg {
				listMsg.Append(msg)
			}
		}
	}

	lgc.answer(listMsg)
	//lgc.privateAnswer(listMsg)
	//lgc.textReply("该命令较为刷屏，已通过私聊发送")

}

func (lgc *LspGroupCommand) RollCommand() {
	msg := lgc.msg
	groupCode := msg.GroupCode

	log := logger.WithField("GroupCode", groupCode)
	log.Info("run roll command")
	defer log.Info("roll command end")

	var rollCmd struct {
		RangeArg string `arg:"" optional:"" help:"roll range, eg. 100 / 50-100"`
	}
	lgc.parseArgs(&rollCmd, "roll")
	if lgc.exit {
		return
	}

	var (
		max int64 = 100
		min int64 = 1
		err error
	)

	rollarg := rollCmd.RangeArg
	if strings.Contains(rollarg, "-") {
		rolls := strings.Split(rollarg, "-")
		if len(rolls) != 2 {
			lgc.textReply(fmt.Sprintf("参数解析错误 - %v", rollarg))
			return
		}
		min, err = strconv.ParseInt(rolls[0], 10, 64)
		if err != nil {
			lgc.textReply(fmt.Sprintf("参数解析错误 - %v", rollarg))
			return
		}
		max, err = strconv.ParseInt(rolls[1], 10, 64)
		if err != nil {
			lgc.textReply(fmt.Sprintf("参数解析错误 - %v", rollarg))
			return
		}
	} else {
		max, err = strconv.ParseInt(rollarg, 10, 64)
		if err != nil {
			lgc.textReply(fmt.Sprintf("参数解析错误 - %v", rollarg))
			return
		}
	}
	if min > max {
		lgc.textReply(fmt.Sprintf("参数解析错误 - %v", rollarg))
		return
	}
	result := rand.Int63n(max-min+1) + min
	log = log.WithField("roll", result)
	lgc.textReply(strconv.FormatInt(result, 10))
}

func (lgc *LspGroupCommand) CheckinCommand() {
	var checkinCmd struct{}
	lgc.parseArgs(&checkinCmd, "签到")
	if lgc.exit {
		return
	}

	msg := lgc.msg
	groupCode := msg.GroupCode

	log := logger.WithField("GroupCode", groupCode)
	log.Infof("run checkin command")
	defer log.Info("checkin command end")

	db, err := localdb.GetClient()
	if err != nil {
		logger.Errorf("get db failed %v", err)
		return
	}
	date := time.Now().Format("20060102")

	err = db.Update(func(tx *buntdb.Tx) error {
		var score int64
		key := localdb.Key("Score", groupCode, msg.Sender.Uin)
		dateMarker := localdb.Key("ScoreDate", groupCode, msg.Sender.Uin, date, nil)

		_, err := tx.Get(dateMarker)
		if err != buntdb.ErrNotFound {
			lgc.textReply("明天再来吧")
			return nil
		}

		val, err := tx.Get(key)
		if err == buntdb.ErrNotFound {
			score = 0
		} else {
			score, err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				log.WithField("value", val).Errorf("parse score failed %v", err)
				return err
			}
		}
		score += 1
		_, _, err = tx.Set(key, strconv.FormatInt(score, 10), nil)
		if err != nil {
			log.WithField("sender", msg.Sender.Uin).Errorf("update score failed %v", err)
			return err
		}

		_, _, err = tx.Set(dateMarker, "1", nil)
		if err != nil {
			log.WithField("sender", msg.Sender.Uin).Errorf("update score marker failed %v", err)
			return err
		}
		lgc.textReply(fmt.Sprintf("签到成功！获得1积分，当前积分为%v", score))
		return nil
	})
	if err != nil {
		log.Errorf("签到失败")
	}
}

func (lgc *LspGroupCommand) ImageContent() {
	msg := lgc.msg
	bot := lgc.bot

	groupCode := msg.GroupCode

	log := logger.WithField("GroupCode", groupCode)

	for _, e := range msg.Elements {
		if e.Type() == message.Image {
			if img, ok := e.(*message.ImageElement); ok {
				rating := lgc.l.checkImage(img)
				if rating == aliyun.SceneSexy {
					sendingMsg := message.NewSendingMessage()
					sendingMsg.Append(message.NewReply(msg))
					sendingMsg.Append(message.NewText("就这"))
					bot.SendGroupMessage(groupCode, sendingMsg)
					return
				} else if rating == aliyun.ScenePorn {
					sendingMsg := message.NewSendingMessage()
					sendingMsg.Append(message.NewReply(msg))
					sendingMsg.Append(message.NewText("多发点"))
					bot.SendGroupMessage(groupCode, sendingMsg)
					return
				}
			} else {
				log.Error("can not cast element to GroupImageElement")
			}
		}
	}
}

func (lgc *LspGroupCommand) textReply(text string) {
	msg := lgc.msg
	bot := lgc.bot

	sendingMsg := message.NewSendingMessage()
	sendingMsg.Append(message.NewReply(msg))
	sendingMsg.Append(message.NewText(text))
	bot.SendGroupMessage(msg.GroupCode, sendingMsg)
}

func (lgc *LspGroupCommand) reply(msg *message.SendingMessage) {
	sendingMsg := message.NewSendingMessage()
	sendingMsg.Append(message.NewReply(lgc.msg))
	for _, e := range msg.Elements {
		sendingMsg.Append(e)
	}
	lgc.answer(sendingMsg)
}

func (lgc *LspGroupCommand) answer(msg *message.SendingMessage) {
	lgc.bot.SendGroupMessage(lgc.msg.GroupCode, msg)
}

func (lgc *LspGroupCommand) privateAnswer(msg *message.SendingMessage) {
	uin := lgc.msg.Sender.Uin
	if lgc.msg.Sender.IsFriend {
		lgc.bot.SendPrivateMessage(uin, msg)
	} else {
		lgc.bot.SendTempMessage(lgc.msg.GroupCode, uin, msg)
	}
}

func (lgc *LspGroupCommand) getArgs() []string {
	if lgc.args == nil {
		text := lgc.msg.Elements[0].(*message.TextElement).Content
		lgc.args = strings.Split(text, " ")[1:]
	}
	return lgc.args
}

func (lgc *LspGroupCommand) parseArgs(ast interface{}, name string) {
	args := lgc.getArgs()
	cmdOut := &strings.Builder{}
	k, err := kong.New(ast, kong.Exit(lgc.Exit), kong.Name(name), kong.UsageOnError())
	if err != nil {
		logger.Errorf("kong new failed %v", err)
		lgc.textReply("失败")
		lgc.Exit(0)
		return
	}
	k.Stdout = cmdOut
	_, err = k.Parse(args)
	if lgc.exit {
		logger.WithField("content", args).Debug("exit")
		lgc.textReply(cmdOut.String())
		return
	}
	if err != nil {
		logger.WithField("content", args).Errorf("kong parse failed %v", err)
		lgc.textReply(fmt.Sprintf("失败 - %v", err))
		lgc.Exit(0)
		return
	}
}

func (lgc *LspGroupCommand) parseRawSiteAndType(rawSite string, rawType string) (string, concern.Type, error) {
	var (
		site      string
		_type     string
		found     bool
		watchType concern.Type
		err       error
	)
	rawSite = strings.Trim(rawSite, "\"")
	rawType = strings.Trim(rawType, "\"")
	site, err = lgc.parseRawSite(rawSite)
	if err != nil {
		return "", concern.Empty, err
	}
	_type, found = utils.PrefixMatch([]string{"live", "news"}, rawType)
	if !found {
		return "", concern.Empty, errors.New("can not determine type")
	}

	switch _type {
	case "live":
		if site == bilibili.Site {
			watchType = concern.BibiliLive
		} else if site == douyu.Site {
			watchType = concern.DouyuLive
		} else {
			return "", concern.Empty, errors.New("unknown watch type")
		}
	case "news":
		if site == bilibili.Site {
			watchType = concern.BilibiliNews
		} else {
			return "", concern.Empty, errors.New("unknown watch type")
		}
	default:
		return "", concern.Empty, errors.New("unknown watch type")
	}
	return site, watchType, nil
}

func (lgc *LspGroupCommand) parseRawSite(rawSite string) (string, error) {
	var (
		found bool
		site  string
	)

	site, found = utils.PrefixMatch([]string{bilibili.Site, douyu.Site}, rawSite)
	if !found {
		return "", errors.New("can not determine site")
	}
	return site, nil
}
