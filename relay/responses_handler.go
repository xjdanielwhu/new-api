package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

func isResponsesContentArrayErrorMessage(msg string) bool {
	msg = strings.ToLower(msg)
	return strings.Contains(msg, "invalid 'input[") &&
		strings.Contains(msg, ".content'") &&
		strings.Contains(msg, "array too long")
}

func isResponsesReasoningSummaryErrorMessage(msg string) bool {
	msg = strings.ToLower(msg)
	return strings.Contains(msg, "missing required parameter") &&
		strings.Contains(msg, "input[") &&
		strings.Contains(msg, ".summary'")
}

func flattenResponsesContentArraysForRetry(body []byte) ([]byte, bool) {
	return rewriteResponsesInputForRetry(body, false)
}

func stripResponsesReasoningItemsForRetry(body []byte) ([]byte, bool) {
	return rewriteResponsesInputForRetry(body, true)
}

func rewriteResponsesInputForRetry(body []byte, dropReasoning bool) ([]byte, bool) {
	var reqMap map[string]any
	if err := common.Unmarshal(body, &reqMap); err != nil {
		return body, false
	}
	inputVal, ok := reqMap["input"]
	if !ok {
		return body, false
	}
	inputBytes, err := common.Marshal(inputVal)
	if err != nil {
		return body, false
	}
	var input []map[string]any
	if err := common.Unmarshal(inputBytes, &input); err != nil {
		return body, false
	}
	changed := false
	filtered := make([]map[string]any, 0, len(input))
	for _, item := range input {
		itemType, _ := item["type"].(string)
		if itemType == "reasoning" {
			if dropReasoning {
				changed = true
				continue
			}
			if _, exists := item["content"]; exists {
				delete(item, "content")
				changed = true
			}
			if _, exists := item["summary"]; !exists {
				item["summary"] = []any{}
				changed = true
			}
			filtered = append(filtered, item)
			continue
		}
		if itemType == "message" {
			content, ok := item["content"].([]any)
			if ok {
				texts := make([]string, 0, len(content))
				convertible := true
				for _, partAny := range content {
					part, ok := partAny.(map[string]any)
					if !ok {
						convertible = false
						break
					}
					partType, _ := part["type"].(string)
					switch partType {
					case "input_text", "output_text", "text":
						if s, _ := part["text"].(string); s != "" {
							texts = append(texts, s)
						}
					default:
						convertible = false
					}
				}
				if convertible {
					item["content"] = strings.Join(texts, "\n")
					changed = true
				}
			}
		}
		filtered = append(filtered, item)
	}
	if !changed {
		return body, false
	}
	reqMap["input"] = filtered
	out, err := common.Marshal(reqMap)
	if err != nil {
		return body, false
	}
	return out, true
}

func retryResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, body []byte, logMessage string) (*http.Response, *types.NewAPIError) {
	storage, createErr := common.CreateBodyStorage(body)
	if createErr != nil {
		return nil, nil
	}
	c.Set(common.KeyBodyStorage, storage)
	c.Request.Body = io.NopCloser(storage)
	if info.Request != nil {
		_ = common.Unmarshal(body, info.Request)
	}
	logger.LogWarn(c, logMessage)
	requestBody := bytes.NewBuffer(body)
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil || resp == nil {
		return nil, nil
	}
	httpResp := resp.(*http.Response)
	if httpResp.StatusCode == http.StatusOK {
		return httpResp, nil
	}
	return httpResp, service.RelayErrorHandler(c.Request.Context(), httpResp, false)
}

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		rawBody, err := io.ReadAll(storage)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		requestBody = bytes.NewReader(openai.SanitizeResponsesRequestBody(rawBody))
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}
		jsonData = openai.SanitizeResponsesRequestBody(jsonData)
		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
		requestBody = bytes.NewBuffer(jsonData)
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			storage, storageErr := common.GetBodyStorage(c)
			if storageErr == nil {
				rawBody, readErr := storage.Bytes()
				if readErr == nil && isResponsesContentArrayErrorMessage(newAPIError.Error()) {
					if newBody, changed := flattenResponsesContentArraysForRetry(rawBody); changed {
						retryResp, retryErr := retryResponsesRequest(c, info, adaptor, newBody, "responses handler auto-recovery: flattened input content arrays and retrying upstream once")
						if retryResp != nil && retryErr == nil {
							httpResp = retryResp
							goto RESPONSE_OK
						}
						if retryErr != nil {
							newAPIError = retryErr
							if isResponsesReasoningSummaryErrorMessage(newAPIError.Error()) {
								if newBody2, changed2 := stripResponsesReasoningItemsForRetry(newBody); changed2 {
									retryResp2, retryErr2 := retryResponsesRequest(c, info, adaptor, newBody2, "responses handler auto-recovery: stripped incompatible reasoning items and retrying upstream once")
									if retryResp2 != nil && retryErr2 == nil {
										httpResp = retryResp2
										goto RESPONSE_OK
									}
									if retryErr2 != nil {
										newAPIError = retryErr2
									}
								}
							}
						}
					}
				}
			}
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

RESPONSE_OK:
	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData
		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)
		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
	} else {
		service.PostTextConsumeQuota(c, info, usageDto, nil)
	}
	return nil
}
