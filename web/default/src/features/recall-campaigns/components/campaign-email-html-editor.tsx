import { useRef, useState } from 'react'
import type { FieldPath, UseFormReturn } from 'react-hook-form'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { previewRecallEmail } from '../api'
import { insertRecallEmailAction, RECALL_EMAIL_ACTIONS } from '../helpers'
import type { RecallCampaignDraft } from '../types'

interface CampaignEmailHtmlEditorProps {
  form: UseFormReturn<RecallCampaignDraft>
  index: number
  disabled: boolean
}

interface RecallEmailPreviewFrameProps {
  previewHTML: string
  errorMessage: string
}

function getRecallEditorErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) return error.message
  return 'Recall email preview failed'
}

export function RecallEmailPreviewFrame(
  props: RecallEmailPreviewFrameProps
): React.JSX.Element {
  const { t } = useTranslation()

  return (
    <div className='space-y-2'>
      {props.errorMessage ? (
        <p role='alert' className='text-destructive text-sm'>
          {t(props.errorMessage)}
        </p>
      ) : null}
      {props.previewHTML ? (
        <iframe
          title={t('Recall email preview')}
          sandbox=''
          srcDoc={props.previewHTML}
          className='h-[640px] w-full rounded-md border bg-white'
        />
      ) : null}
    </div>
  )
}

export function CampaignEmailHtmlEditor(
  props: CampaignEmailHtmlEditorProps
): React.JSX.Element {
  const { t } = useTranslation()
  const textareaRef = useRef<HTMLTextAreaElement | null>(null)
  const [previewHTML, setPreviewHTML] = useState('')
  const [latestError, setLatestError] = useState('')
  const subjectPath =
    `email_sequence.${props.index}.templates.en.subject` as FieldPath<RecallCampaignDraft>
  const bodyPath =
    `email_sequence.${props.index}.templates.en.body_html` as FieldPath<RecallCampaignDraft>
  const legacyBodyPath =
    `email_sequence.${props.index}.templates.en.body_text` as FieldPath<RecallCampaignDraft>
  const bodyId = `recall-email-${props.index}-body-html`
  const bodyErrorId = `${bodyId}-error`
  const bodyRegistration = props.form.register(bodyPath)
  const bodyError = props.form.getFieldState(
    bodyPath,
    props.form.formState
  ).error
  const bodyHTML = String(props.form.getValues(bodyPath) ?? '')
  const bodyText = String(props.form.getValues(legacyBodyPath) ?? '')
  const localBodyError =
    !bodyHTML.trim() && !bodyText.trim()
      ? 'Exactly one email body is required'
      : ''
  const activeBodyError = bodyError?.message
    ? String(bodyError.message)
    : localBodyError
  const previewMutation = useMutation({ mutationFn: previewRecallEmail })

  const insertAction = (action: (typeof RECALL_EMAIL_ACTIONS)[number]) => {
    const textarea = textareaRef.current
    const currentValue = String(props.form.getValues(bodyPath) ?? '')
    const start = textarea?.selectionStart ?? currentValue.length
    const end = textarea?.selectionEnd ?? start
    const inserted = insertRecallEmailAction(currentValue, start, end, action)
    props.form.setValue(bodyPath, inserted.value, {
      shouldDirty: true,
      shouldValidate: true,
    })
    const restoreSelection = () => {
      textarea?.focus()
      textarea?.setSelectionRange(inserted.selection, inserted.selection)
    }
    if (typeof requestAnimationFrame === 'function') {
      requestAnimationFrame(restoreSelection)
    } else {
      restoreSelection()
    }
  }

  const previewEmail = async () => {
    const valid = await props.form.trigger([subjectPath, bodyPath])
    if (!valid) return
    try {
      setLatestError('')
      const response = await previewMutation.mutateAsync({
        template: {
          subject: String(props.form.getValues(subjectPath) ?? ''),
          body_html: String(props.form.getValues(bodyPath) ?? ''),
        },
      })
      setPreviewHTML(response.data?.body_html ?? '')
    } catch (error) {
      setLatestError(getRecallEditorErrorMessage(error))
    }
  }

  return (
    <div className='space-y-3'>
      <div className='space-y-2'>
        <Label htmlFor={bodyId}>{t('Body HTML')}</Label>
        <Textarea
          id={bodyId}
          rows={14}
          disabled={props.disabled}
          aria-invalid={Boolean(activeBodyError)}
          aria-describedby={activeBodyError ? bodyErrorId : undefined}
          {...bodyRegistration}
          defaultValue={bodyHTML}
          ref={(element) => {
            bodyRegistration.ref(element)
            textareaRef.current = element
          }}
        />
        {activeBodyError ? (
          <p id={bodyErrorId} role='alert' className='text-destructive text-sm'>
            {t(activeBodyError)}
          </p>
        ) : null}
      </div>
      <div className='flex flex-wrap gap-2'>
        {RECALL_EMAIL_ACTIONS.map((action) => (
          <Button
            aria-label={t('Insert {{action}}', { action })}
            disabled={props.disabled}
            key={action}
            type='button'
            variant='outline'
            onClick={() => insertAction(action)}
          >
            {action}
          </Button>
        ))}
      </div>
      <Button
        aria-label={t('Recall email preview')}
        disabled={props.disabled || previewMutation.isPending}
        type='button'
        variant='outline'
        onClick={() => void previewEmail()}
      >
        {previewMutation.isPending ? t('Previewing') : t('Preview email')}
      </Button>
      <RecallEmailPreviewFrame
        previewHTML={previewHTML}
        errorMessage={latestError}
      />
    </div>
  )
}
