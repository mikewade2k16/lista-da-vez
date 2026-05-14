<script setup lang="ts">
import AdminPageHeader from '../components/admin/AdminPageHeader.vue'
import ThemeStudioDetailedGrid from '../components/theme/studio/ThemeStudioDetailedGrid.vue'
import ThemeStudioHeaderControls from '../components/theme/studio/ThemeStudioHeaderControls.vue'
import ThemeStudioSimplePanel from '../components/theme/studio/ThemeStudioSimplePanel.vue'
import { useThemeStudio } from '../composables/useThemeStudio'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'themes',
  pageLabel: 'Theme Studio'
})

const {
  search,
  selectedTheme,
  showDetailedEditors,
  customThemeNameDraft,
  hasCustomTheme,
  fontItems,
  isSimpleMode,
  themeOrder,
  themeLabels,
  groupedSections,
  variableValueMap,
  radiusValueMap,
  tripletModelValueMap,
  headerState,
  pageHeaderState,
  radiusKeys,
  shadowColorKeys,
  headerTextLinkValue,
  toGroupLabel,
  selectTheme,
  applySelectedTheme,
  resetSelectedThemeOverrides,
  createCustomFromSelected,
  saveCustomThemeName,
  setCustomThemeNameDraft,
  setSearch,
  setShowDetailedEditors,
  onVariableTextValue,
  onTripletColorValueChange,
  onCssValueChange,
  onRadiusChange,
  onFontChange,
  onHeaderBackgroundChange,
  onHeaderPanelBackgroundChange,
  onHeaderBackgroundLinkChange,
  onHeaderBrandBlurChange,
  onHeaderPanelBlurChange,
  onHeaderBlurLinkChange,
  onHeaderDividerChange,
  onHeaderSeparatorChange,
  onHeaderDividerLinkChange,
  onHeaderTextFollowChange,
  onHeaderTextChange,
  onHeaderActionPrimaryChange,
  onHeaderInteractionChange,
  onHeaderActiveChange,
  onHeaderInteractionLinkChange,
  onHeaderShellShadowChange,
  onHeaderFadeTopChange,
  onHeaderFadeBottomChange,
  onHeaderFadeTopSizeChange,
  onHeaderFadeBottomSizeChange,
  onPageHeaderEyebrowVisibilityChange,
  onPageHeaderTitleVisibilityChange,
  onPageHeaderDescriptionVisibilityChange,
  onPageHeaderDisableAll
} = useThemeStudio()

function toPxNumber(value: string) {
  const parsed = Number.parseFloat(String(value ?? '').replace('px', '').trim())
  if (!Number.isFinite(parsed) || parsed < 0) return 0
  return Math.round(parsed)
}
</script>

<template>
  <section class="space-y-4">
    <AdminPageHeader
      eyebrow="Admin"
      title="Theme Studio"
      description="Modo simples por padrao para editar o tema inteiro. Ative o modo detalhado para customizar token por token."
    />

    <ThemeStudioHeaderControls :theme-order="themeOrder" :theme-labels="themeLabels" :selected-theme="selectedTheme"
      :has-custom-theme="hasCustomTheme" :custom-theme-name-draft="customThemeNameDraft"
      :show-detailed-editors="showDetailedEditors" :search="search" @update:selected-theme="selectTheme"
      @apply-selected-theme="applySelectedTheme" @reset-selected-theme="resetSelectedThemeOverrides"
      @update:custom-theme-name-draft="setCustomThemeNameDraft" @save-custom-theme-name="saveCustomThemeName"
      @create-custom-from-selected="createCustomFromSelected" @update:show-detailed-editors="setShowDetailedEditors"
      @update:search="setSearch" />

    <ThemeStudioSimplePanel v-if="isSimpleMode" :font-items="fontItems"
      :font-value="variableValueMap['font-sans'] ?? ''" :radius-keys="radiusKeys" :radius-value-map="radiusValueMap"
      :shadow-color="variableValueMap['shadow-color'] ?? ''"
      :shadow-glow-color="variableValueMap['shadow-glow-color'] ?? ''" :accent-values="{
        primary: tripletModelValueMap['primary'] ?? '',
        primary600: tripletModelValueMap['primary-600'] ?? '',
        success: tripletModelValueMap['success'] ?? '',
        danger: tripletModelValueMap['danger'] ?? '',
        ring: tripletModelValueMap['ring'] ?? ''
      }" :header-values="{
        brandBg: variableValueMap['admin-header-brand-bg'] ?? '',
        panelBg: variableValueMap['admin-header-panel-bg'] ?? '',
        brandBlur: toPxNumber(variableValueMap['admin-header-brand-blur'] ?? '0'),
        panelBlur: toPxNumber(variableValueMap['admin-header-panel-blur'] ?? '0'),
        border: variableValueMap['admin-header-border'] ?? '',
        separator: variableValueMap['admin-header-separator'] ?? '',
        text: variableValueMap['admin-header-text'] ?? '',
        hover: variableValueMap['admin-header-hover-bg'] ?? '',
        active: variableValueMap['admin-header-active-bg'] ?? '',
        shellShadow: variableValueMap['admin-header-shell-shadow'] ?? '',
        fadeTop: variableValueMap['admin-header-fade-top'] ?? '',
        fadeBottom: variableValueMap['admin-header-fade-bottom'] ?? '',
        fadeTopSize: toPxNumber(variableValueMap['admin-header-fade-top-size'] ?? '0'),
        fadeBottomSize: toPxNumber(variableValueMap['admin-header-fade-bottom-size'] ?? '0')
      }" :header-state="headerState" :page-header-state="pageHeaderState" :header-text-link-value="headerTextLinkValue"
      @update-font="onFontChange" @update-radius="onRadiusChange($event.key, $event.value)"
      @update-shadow-color="onCssValueChange('shadow-color', $event)"
      @update-shadow-glow-color="onCssValueChange('shadow-glow-color', $event)"
      @update-accent="onTripletColorValueChange($event.key, $event.value)"
      @update-header-brand-bg="onHeaderBackgroundChange" @update-header-panel-bg="onHeaderPanelBackgroundChange"
      @toggle-header-backgrounds="onHeaderBackgroundLinkChange" @update-header-brand-blur="onHeaderBrandBlurChange"
      @update-header-panel-blur="onHeaderPanelBlurChange" @toggle-header-blurs="onHeaderBlurLinkChange"
      @update-header-border="onHeaderDividerChange" @update-header-separator="onHeaderSeparatorChange"
      @toggle-header-dividers="onHeaderDividerLinkChange" @toggle-header-text-follow="onHeaderTextFollowChange"
      @update-header-text="onHeaderTextChange" @toggle-header-primary-actions="onHeaderActionPrimaryChange"
      @toggle-header-interactions="onHeaderInteractionLinkChange" @update-header-hover="onHeaderInteractionChange"
      @update-header-active="onHeaderActiveChange" @update-header-shell-shadow="onHeaderShellShadowChange"
      @update-header-fade-top="onHeaderFadeTopChange" @update-header-fade-bottom="onHeaderFadeBottomChange"
      @update-header-fade-top-size="onHeaderFadeTopSizeChange"
      @update-header-fade-bottom-size="onHeaderFadeBottomSizeChange"
      @toggle-page-header-eyebrow="onPageHeaderEyebrowVisibilityChange"
      @toggle-page-header-title="onPageHeaderTitleVisibilityChange"
      @toggle-page-header-description="onPageHeaderDescriptionVisibilityChange"
      @disable-page-header-all="onPageHeaderDisableAll" />

    <ThemeStudioDetailedGrid v-else :sections="groupedSections" :font-items="fontItems" :group-label-by="toGroupLabel"
      :variable-value-map="variableValueMap" :radius-value-map="radiusValueMap"
      :triplet-model-value-map="tripletModelValueMap" :radius-keys="radiusKeys" :shadow-color-keys="shadowColorKeys"
      @update-font="onFontChange" @update-radius="onRadiusChange($event.key, $event.value)"
      @update-css="onCssValueChange($event.key, $event.value)"
      @update-triplet="onTripletColorValueChange($event.key, $event.value)"
      @update-text="onVariableTextValue($event.key, $event.value)" />
  </section>
</template>
