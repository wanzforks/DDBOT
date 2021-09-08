package huya

import (
	"github.com/Sora233/DDBOT/lsp/concern"
)

type GroupConcernConfig struct {
	concern.GroupConcernConfig
}

func (g *GroupConcernConfig) AtBeforeHook(notify concern.Notify) (hook *concern.HookResult) {
	hook = new(concern.HookResult)
	switch e := notify.(type) {
	case *ConcernLiveNotify:
		if !e.Living {
			hook.Reason = "Living is false"
		} else if !notify.(*ConcernLiveNotify).LiveStatusChanged {
			hook.Reason = "Living ok but LiveStatusChanged is false"
		} else {
			hook.Pass = true
		}
	default:
		hook.Reason = "default"
	}
	return
}

func (g *GroupConcernConfig) ShouldSendHook(notify concern.Notify) (hook *concern.HookResult) {
	hook = new(concern.HookResult)
	switch e := notify.(type) {
	case *ConcernLiveNotify:
		if e.Living {
			if e.LiveStatusChanged {
				// 上播了
				hook.Pass = true
				return
			}
			if e.LiveTitleChanged {
				// 直播间标题改了，检查改标题推送配置
				hook.PassOrReason(g.GroupConcernNotify.CheckTitleChangeNotify(notify.Type()), "CheckTitleChangeNotify is false")
				return
			}
		} else {
			if e.LiveStatusChanged {
				// 下播了，检查下播推送配置
				hook.PassOrReason(g.GroupConcernNotify.CheckOfflineNotify(notify.Type()), "CheckOfflineNotify is false")
				return
			}
		}
	}
	return g.GroupConcernConfig.ShouldSendHook(notify)
}

func NewGroupConcernConfig(g *concern.GroupConcernConfig) *GroupConcernConfig {
	return &GroupConcernConfig{*g}
}
