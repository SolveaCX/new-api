/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import i18next from 'i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { useOnboardingStore } from '@/stores/onboarding-store'
import { useCanUseGroups } from '@/hooks/use-enterprise'
import { useSystemConfig } from '@/hooks/use-system-config'
import { getUserModels, getUserGroups } from './api'
import { PlaygroundChat } from './components/playground-chat'
import { FirstRunWelcome, GetKeyCard } from './components/playground-first-run'
import { PlaygroundInput } from './components/playground-input'
import { MESSAGE_ROLES, MESSAGE_STATUS } from './constants'
import { usePlaygroundState, useChatHandler } from './hooks'
import {
  createUserMessage,
  createLoadingAssistantMessage,
  getFirstRunChatOverride as resolveFirstRunChatOverride,
  pickFirstRunModel,
  shouldOpenFirstRunTopupPrompt,
  clearFirstRunDone,
  isFirstRunActive,
  markFirstRunDone,
  markFirstRunStarted,
} from './lib'
import type { Message as MessageType } from './types'

// PLG users are always pinned to the single `plg` group.
const PLG_GROUP = 'plg'

export function Playground({
  firstRun: firstRunFromUrl = false,
}: {
  firstRun?: boolean
}) {
  const navigate = useNavigate()
  const canUseGroups = useCanUseGroups()
  const { playgroundDefaultModel, enableStripeCardBind } = useSystemConfig()
  const authUser = useAuthStore((state) => state.auth.user)
  const openOnboarding = useOnboardingStore((state) => state.openOnboarding)

  // The onboarding is triggered one-shot via `?first=1`, but it must persist
  // across tab switches / reloads for a brand-new user until they finish their
  // first successful call. We remember that per user (keyed on the authed id so
  // a shared browser never leaks state between accounts): a fresh `?first=1`
  // (re-)starts it, completion marks it done. The effective `firstRun` below
  // therefore stays true on later returns until the user has completed once, so
  // every downstream `firstRun` usage is unchanged.
  const userId = authUser?.id
  const firstRun = firstRunFromUrl || isFirstRunActive(userId)

  // Persist first-run entry. Explicit `?first=1` re-enables onboarding even if
  // the user completed it before (clears the done flag), then records that the
  // user has started so a plain tab return keeps showing it.
  useEffect(() => {
    if (!firstRunFromUrl) return
    if (userId === undefined) return
    clearFirstRunDone(userId)
    markFirstRunStarted(userId)
  }, [firstRunFromUrl, userId])
  const {
    config,
    parameterEnabled,
    messages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
    minimalParameters: firstRun,
  })

  // Edit dialog state
  const [editingMessageKey, setEditingMessageKey] = useState<string | null>(
    null
  )

  // --- First-run onboarding state (?first=1) ---
  // Whether the "get your API key" card is currently visible. Shown once per
  // session after the first successful assistant response, then dismissed.
  const [showGetKeyCard, setShowGetKeyCard] = useState(false)
  // Whether the user has actually sent a message during THIS first-run session.
  // The get-key card keys off this (not raw messages) so a stale localStorage
  // conversation can't prematurely surface it.
  const [sentThisSession, setSentThisSession] = useState(false)
  // Guards so one-shot first-run cards fire at most once per session (refs
  // survive re-renders without retriggering effects).
  const clearedFirstRunMessagesRef = useRef(false)
  const getKeyCardShownRef = useRef(false)
  const topupPromptShownRef = useRef(false)
  const [userPickedModel, setUserPickedModel] = useState(false)

  // Initialize first-run mode once on mount. The clean slate matters because a
  // just-registered user may be in a browser that still holds a previous
  // account's persisted conversation, which would otherwise suppress the welcome
  // banner (gated on messages.length === 0).
  useEffect(() => {
    if (!firstRun) return
    if (clearedFirstRunMessagesRef.current) return
    clearedFirstRunMessagesRef.current = true
    if (messages.length > 0) updateMessages([])
  }, [firstRun, messages.length, updateMessages])

  // Load models
  const { data: modelsData, isLoading: isLoadingModels } = useQuery({
    queryKey: ['playground-models', config.group],
    queryFn: async () => {
      try {
        return await getUserModels(config.group)
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : i18next.t('Failed to load playground models')
        )
        return []
      }
    },
  })

  // Load groups only when the current user can choose token groups.
  const { data: groupsData } = useQuery({
    queryKey: ['playground-groups'],
    queryFn: async () => {
      try {
        return await getUserGroups()
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : i18next.t('Failed to load playground groups')
        )
        return []
      }
    },
    enabled: canUseGroups,
  })

  const firstRunModel = useMemo(() => {
    if (!firstRun || !modelsData?.length) return undefined
    return pickFirstRunModel(modelsData, playgroundDefaultModel)
  }, [firstRun, modelsData, playgroundDefaultModel])

  const isCurrentModelValid =
    !!config.model &&
    !!modelsData?.some((model) => model.value === config.model)
  const isFirstRunModelApplied =
    !!firstRunModel &&
    isCurrentModelValid &&
    (userPickedModel || config.model === firstRunModel)
  const isFirstRunModelReady = !firstRun || isFirstRunModelApplied
  const getFirstRunChatOverride = useCallback(
    () =>
      resolveFirstRunChatOverride({
        firstRun,
        firstRunModel,
        currentModel: config.model,
        userPickedModel,
      }),
    [firstRun, firstRunModel, config.model, userPickedModel]
  )

  // PLG users are pinned to the `plg` group so model fetching uses it.
  useEffect(() => {
    if (authUser && !canUseGroups && config.group !== PLG_GROUP) {
      updateConfig('group', PLG_GROUP)
    }
  }, [authUser, canUseGroups, config.group, updateConfig])

  // Update models when data changes
  useEffect(() => {
    if (!modelsData) return

    setModels(modelsData)

    if (firstRun && !userPickedModel && !!firstRunModel) {
      if (config.model === firstRunModel) return
      updateConfig('model', firstRunModel)
      return
    }

    // Set default model if current model is not available
    const isCurrentModelValid = modelsData.some((m) => m.value === config.model)
    if (!isCurrentModelValid) {
      updateConfig('model', modelsData[0]?.value ?? '')
    }
  }, [
    modelsData,
    config.model,
    firstRun,
    firstRunModel,
    userPickedModel,
    setModels,
    updateConfig,
  ])

  // Update groups when data changes
  useEffect(() => {
    if (!groupsData) return

    setGroups(groupsData)

    const hasCurrentGroup = groupsData.some((g) => g.value === config.group)
    if (!hasCurrentGroup && groupsData.length > 0) {
      const fallback =
        groupsData.find((g) => g.value === 'default')?.value ??
        groupsData[0].value
      updateConfig('group', fallback)
    }
  }, [groupsData, setGroups, config.group, updateConfig])

  // Detect the first successful assistant response in first-run mode and slide
  // in the "get your API key" card once per session.
  const hasCompletedAssistant = useMemo(
    () =>
      messages.some(
        (m) =>
          m.from === MESSAGE_ROLES.ASSISTANT &&
          m.status === MESSAGE_STATUS.COMPLETE &&
          !!m.versions?.[0]?.content?.trim()
      ),
    [messages]
  )

  useEffect(() => {
    if (!firstRun) return
    if (getKeyCardShownRef.current) return
    // Require a real send this session so a restored conversation can't trigger
    // the card before the user has actually made a call.
    if (!sentThisSession) return
    if (!hasCompletedAssistant) return
    getKeyCardShownRef.current = true
    // First successful call: mark onboarding done in persistent storage so a
    // later tab return / reload no longer reshows the welcome banner for this
    // user (the effective firstRun then resolves to false).
    markFirstRunDone(userId)
    const showCardTimer = window.setTimeout(() => {
      setShowGetKeyCard(true)
      // First call succeeded — drop `?first=1` from the URL so a reload/back-nav
      // doesn't replay the one-shot onboarding (welcome banner + model force).
      // The card is driven by showGetKeyCard state, so it stays after firstRun flips.
      navigate({ to: '/playground', replace: true })
    }, 0)
    return () => window.clearTimeout(showCardTimer)
  }, [firstRun, sentThisSession, hasCompletedAssistant, navigate, userId])

  useEffect(() => {
    const shouldOpen = shouldOpenFirstRunTopupPrompt({
      firstRun,
      sentThisSession,
      hasCompletedAssistant,
      promptShown: topupPromptShownRef.current,
      enableStripeCardBind,
      stripeCardBound: authUser?.stripe_card_bound,
    })
    if (!shouldOpen) return

    topupPromptShownRef.current = true
    openOnboarding()
  }, [
    firstRun,
    sentThisSession,
    hasCompletedAssistant,
    enableStripeCardBind,
    authUser?.stripe_card_bound,
    openOnboarding,
  ])

  const prepareFirstRunSend = useCallback(() => {
    if (firstRun && !isFirstRunModelApplied) {
      toast.error(i18next.t('Failed to load playground models'))
      return false
    }
    if (firstRun) setSentThisSession(true)
    return true
  }, [firstRun, isFirstRunModelApplied])

  const handleSendMessage = (text: string, model?: string) => {
    if (!prepareFirstRunSend()) return
    const userMessage = createUserMessage(text)
    const assistantMessage = createLoadingAssistantMessage()

    const newMessages = [...messages, userMessage, assistantMessage]
    updateMessages(newMessages)

    // An example prompt can force a specific model (e.g. the image-generation
    // chip pins `gemini-2.5-flash-image`). Persist the selection so the picker
    // reflects it, and mark it as an explicit user choice so the first-run cheap
    // default never overrides it. Crucially, we ALSO pass the model as a direct
    // send override here: `updateConfig` is async and wouldn't be reflected in
    // `config` for this same-tick send, so the override guarantees THIS message
    // is actually requested against the forced model.
    if (model) {
      setUserPickedModel(true)
      updateConfig('model', model)
      sendChat(newMessages, { model })
      return
    }

    // Send chat request
    sendChat(newMessages, getFirstRunChatOverride())
  }

  const handleCopyMessage = (message: MessageType) => {
    // Copy is handled in MessageActions component
    // eslint-disable-next-line no-console
    console.log('Message copied:', message.key)
  }

  const handleRegenerateMessage = (message: MessageType) => {
    // Find the message index and regenerate from there
    const messageIndex = messages.findIndex((m) => m.key === message.key)
    if (messageIndex === -1) return

    // Remove messages after this one and regenerate
    const messagesUpToHere = messages.slice(0, messageIndex)
    const loadingMessage = createLoadingAssistantMessage()
    const newMessages = [...messagesUpToHere, loadingMessage]

    updateMessages(newMessages)
    sendChat(newMessages, getFirstRunChatOverride())
  }

  const handleEditMessage = useCallback((message: MessageType) => {
    setEditingMessageKey(message.key)
  }, [])

  const handleEditOpenChange = useCallback((open: boolean) => {
    if (!open) setEditingMessageKey(null)
  }, [])

  // Apply edit and optionally re-submit from the edited user message
  const applyEdit = useCallback(
    (newContent: string, submit: boolean) => {
      if (!editingMessageKey) return
      const index = messages.findIndex((m) => m.key === editingMessageKey)
      if (index === -1) return

      const updated = messages.map((m) =>
        m.key === editingMessageKey
          ? { ...m, versions: [{ ...m.versions[0], content: newContent }] }
          : m
      )

      setEditingMessageKey(null)

      if (!submit || updated[index].from !== 'user') {
        updateMessages(updated)
        return
      }

      const toSubmit = [
        ...updated.slice(0, index + 1),
        createLoadingAssistantMessage(),
      ]
      if (!prepareFirstRunSend()) return
      updateMessages(toSubmit)
      sendChat(toSubmit, getFirstRunChatOverride())
    },
    [
      editingMessageKey,
      messages,
      prepareFirstRunSend,
      updateMessages,
      sendChat,
      getFirstRunChatOverride,
    ]
  )

  const handleDeleteMessage = (message: MessageType) => {
    const newMessages = messages.filter((m) => m.key !== message.key)
    updateMessages(newMessages)
  }

  return (
    <div className='relative flex size-full flex-col overflow-hidden'>
      {/* Welcome banner + example prompts — shown on an empty Playground for
          every user (new users get the first-run banner, returning users get a
          neutral "try one of these" header with the same one-click prompts). */}
      {messages.length === 0 && (
        <FirstRunWelcome
          firstRun={firstRun}
          disabled={!isFirstRunModelReady}
          onPickExample={handleSendMessage}
        />
      )}
      {/* Full-width scroll container: scrolling works even over side whitespace */}
      <div className='flex flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={messages}
          onCopyMessage={handleCopyMessage}
          onRegenerateMessage={handleRegenerateMessage}
          onEditMessage={handleEditMessage}
          onDeleteMessage={handleDeleteMessage}
          isGenerating={isGenerating}
          editingKey={editingMessageKey}
          onCancelEdit={handleEditOpenChange}
          onSaveEdit={(newContent) => applyEdit(newContent, false)}
          onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
        />
      </div>

      {/* "Get your API key" card after the first successful response */}
      {showGetKeyCard && (
        <GetKeyCard onDismiss={() => setShowGetKeyCard(false)} />
      )}

      {/* Input area: center content and constrain to the same container width */}
      <div className='mx-auto w-full max-w-4xl'>
        <PlaygroundInput
          disabled={isGenerating}
          submitDisabled={!isFirstRunModelReady}
          showGroupSelector={canUseGroups}
          groups={groups}
          groupValue={config.group}
          isGenerating={isGenerating}
          isModelLoading={isLoadingModels}
          modelValue={config.model}
          models={models}
          onGroupChange={(value) => updateConfig('group', value)}
          onModelChange={(value) => {
            // Mark that the user explicitly chose a model so the first-run cheap
            // default never overrides their choice.
            setUserPickedModel(true)
            updateConfig('model', value)
          }}
          onStop={stopGeneration}
          onSubmit={handleSendMessage}
        />
      </div>
    </div>
  )
}
