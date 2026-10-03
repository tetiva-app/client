package dto

import "github.com/tetiva-app/client/internal/domain/entities"

type UpdateStateDTO struct {
	Phase    string `json:"phase"`
	Version  string `json:"version"`
	Current  string `json:"current"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Install  string `json:"install"`
	Reason   string `json:"reason"`
}

type RestoreTabDTO struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type RestoreTabsDTO struct {
	WorkspaceID string          `json:"workspaceId"`
	Tabs        []RestoreTabDTO `json:"tabs"`
	ActiveTabID string          `json:"activeTabId"`
}

func UpdateStateToDTO(st entities.UpdateState) UpdateStateDTO {
	return UpdateStateDTO{
		Phase:    string(st.Phase),
		Version:  st.Version,
		Current:  st.Current,
		Received: st.Received,
		Total:    st.Total,
		Install:  string(st.Install),
		Reason:   st.Reason,
	}
}

func RestoreTabsToEntity(d RestoreTabsDTO) entities.RestoreTabs {
	tabs := make([]entities.RestoreTab, len(d.Tabs))
	for i, t := range d.Tabs {
		tabs[i] = entities.RestoreTab{Type: t.Type, ID: t.ID}
	}
	return entities.RestoreTabs{WorkspaceID: d.WorkspaceID, Tabs: tabs, ActiveTabID: d.ActiveTabID}
}

func RestoreTabsToDTO(r entities.RestoreTabs) RestoreTabsDTO {
	tabs := make([]RestoreTabDTO, len(r.Tabs))
	for i, t := range r.Tabs {
		tabs[i] = RestoreTabDTO{Type: t.Type, ID: t.ID}
	}
	return RestoreTabsDTO{WorkspaceID: r.WorkspaceID, Tabs: tabs, ActiveTabID: r.ActiveTabID}
}
