# https://api-docs.deepseek.com/quick_start/rate_limit

Rate Limit & Isolation
Concurrency Limit
For each account, the concurrency limits for different DeepSeek API models are shown in the table below.

If you need higher concurrency, you can submit a capacity expansion request. We will match the appropriate concurrency based on your actual business needs. There is no additional cost for capacity expansion.

|                   | deepseek-v4-pro |	deepseek-v4-flash |
| Concurrency Limit |	500           | 	2500          | 

A request counts as one concurrent connection from the time it is sent until the model response is complete
Concurrency limits are calculated at the account level, regardless of which API Key is used
For a given account, API requests within the concurrency limit will receive a response; when the concurrency limit is exceeded, you will receive an HTTP 429 error code
user_id Isolation
You can pass the user_id parameter to the API to achieve fine-grained management of different users on your business side under the same account. The specific functions of user_id are as follows:

Content Safety Isolation: user_id is used to distinguish user identities on your business side for content safety handling
KVCache Isolation: user_id is used to isolate KVCache for users on your business side for privacy management
Scheduling Isolation: user_id is used for scheduling isolation of users on your business side
For regular API users, all user_id values are combined for concurrency limit calculation
For API users with increased concurrency quotas, we will limit the total concurrency under your account, and we will also impose concurrency limits on each user_id you pass (an empty id is treated as a special user_id). For each user_id, the concurrency limit for deepseek-v4-pro is 500, and for deepseek-v4-flash is 2500. If a user_id exceeds its limit, requests with that user_id under your account will receive an HTTP 429 error code
Setting user_id
The user_id parameter must be a string matching the regex [a-zA-Z0-9\-_]+, with a maximum length of 512. Do not include user privacy information in user_id.

You can set the user_id parameter in the following ways:

OpenAI Chat Completions API
HTTP request body:

{
    "model": "deepseek-v4-pro",
    "messages": {"role": "user", "content": "Hello!"},
    "user_id": "your_user_id"
}

If you are using the OpenAI SDK, you need to place the user_id parameter under the extra_body parameter:

response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=[{"role": "user", "content": "Hello!"}],
    extra_body={"user_id": "your_user_id"}
)

Anthropic API
HTTP request body:

{
    "model": "deepseek-v4-pro",
    "messages": {"role": "user", "content": "Hello!"},
    "metadata": {"user_id": "your_user_id"},
    "max_tokens": 1024
}

If you are using the Anthropic SDK, the calling method is as follows:

message = client.messages.create(
    model="deepseek-v4-pro",
    messages=[{"role": "user", "type": "text", "content": "Hello!"}],
    metadata={"user_id": "your_user_id"},
    max_tokens=1024
)


Request Keep-Alive Mechanism
After your request is sent, it may sometimes take a while to receive a response from the server. During this period, your HTTP request will remain connected, and you may continuously receive contents in the following formats:

Non-streaming requests: Continuously return empty lines
Streaming requests: Continuously return SSE keep-alive comments (: keep-alive)
These contents do not affect the parsing of the JSON body of the response. If you are parsing the HTTP responses yourself, please ensure to handle these empty lines or comments appropriately.

If the request has not started inference after 10 minutes, the server will close the connection.

# https://api-docs.deepseek.com/quick_start/error_codes

Error Codes
When calling DeepSeek API, you may encounter errors. Here list the causes and solutions.

                    CODE                    	DESCRIPTION
400 - Invalid Format	Cause: Invalid request body format.
Solution: Please modify your request body according to the hints in the error message. For more API format details, please refer to DeepSeek API Docs.
401 - Authentication Fails	Cause: Authentication fails due to the wrong API key.
Solution: Please check your API key. If you don't have one, please create an API key first.
402 - Insufficient Balance	Cause: You have run out of balance.
Solution: Please check your account's balance, and go to the Top up page to add funds.
422 - Invalid Parameters	Cause: Your request contains invalid parameters.
Solution: Please modify your request parameters according to the hints in the error message. For more API format details, please refer to DeepSeek API Docs.
429 - Rate Limit Reached	Cause: You are sending requests too quickly.
Solution: Please pace your requests reasonably. We also advise users to temporarily switch to the APIs of alternative LLM service providers, like OpenAI.
500 - Server Error	Cause: Our server encounters an issue.
Solution: Please retry your request after a brief wait and contact us if the issue persists.
503 - Server Overloaded	Cause: The server is overloaded due to high traffic.
Solution: Please retry your request after a brief wait.

# https://api-docs.deepseek.com/guides/thinking_mode

Thinking Mode
The DeepSeek model supports the thinking mode: before outputting the final answer, the model will first output a chain-of-thought reasoning to improve the accuracy of the final response.

Thinking Mode Toggle and Effort Control
Control Parameter (OpenAI Format)	Control Parameter (Anthropic Format)
Thinking Mode Toggle(1)	{"thinking": {"type": "enabled/disabled"}}
Thinking Effort Control(2)(3)	{"reasoning_effort": "high/max"}	{"output_config": {"effort": "high/max"}}
(1) The thinking toggle defaults to enabled
(2) In thinking mode, the default effort is high for regular requests; for some complex agent requests (such as Claude Code, OpenCode), effort is automatically set to max
(3) In thinking mode, for compatibility, low and medium are mapped to high, and xhigh is mapped to max

When using the OpenAI SDK, you need to pass the thinking parameter within extra_body:

response = client.chat.completions.create(
  model="deepseek-v4-pro",
  # ...
  reasoning_effort="high",
  extra_body={"thinking": {"type": "enabled"}}
)

Input and Output Parameters
Thinking mode does not support the temperature, top_p, presence_penalty, or frequency_penalty parameters. Please note that, for compatibility with existing software, setting these parameters will not trigger an error but will also have no effect.

In thinking mode, the chain-of-thought content is returned via the reasoning_content parameter, at the same level as content. When concatenating subsequent turns, you can selectively return reasoning_content to the API:

Between two user messages, if the model did not perform a tool call, the intermediate assistant's reasoning_content does not need to participate in the context concatenation. If passed to the API in subsequent turns, it will be ignored. See Multi-turn Conversation for details.
Between two user messages, if the model performed a tool call, the intermediate assistant's reasoning_content must participate in the context concatenation and must be passed back to the API in all subsequent user interaction turns. See Tool Calls for details.
Multi-turn Conversation
In each turn of the conversation, the model outputs the CoT (reasoning_content) and the final answer (content). If there is no tool call, the CoT content from previous turns will not be concatenated into the context in the next turn, as illustrated in the following diagram:


Sample Code
The following code, using Python as an example, demonstrates how to access the CoT and the final answer, as well as how to concatenate context in multi-turn conversations.

NoStreaming
Streaming
from openai import OpenAI
client = OpenAI(api_key="<DeepSeek API Key>", base_url="https://api.deepseek.com")

# Turn 1
messages = [{"role": "user", "content": "9.11 and 9.8, which is greater?"}]
response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=messages,
    reasoning_effort="high"
    extra_body={"thinking": {"type": "enabled"}},
)

reasoning_content = response.choices[0].message.reasoning_content
content = response.choices[0].message.content

# Turn 2
# The reasoning_content will be ignored by the API
messages.append(response.choices[0].message)
messages.append({'role': 'user', 'content': "How many Rs are there in the word 'strawberry'?"})
response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=messages,
    reasoning_effort="high"
    extra_body={"thinking": {"type": "enabled"}},
)
# ...


Tool Calls
The DeepSeek model's thinking mode supports tool calls. Before outputting the final answer, the model can perform multiple turns of reasoning and tool calls to improve the quality of the response. The calling pattern is illustrated below:


Please note that, unlike turns in thinking mode that do not involve tool calls, for turns that do perform tool calls, the reasoning_content must be fully passed back to the API in all subsequent requests.

If your code does not correctly pass back reasoning_content, the API will return a 400 error. Please refer to the sample code below for the correct approach.

Sample Code
Below is a simple sample code for tool calls in thinking mode:

import os
import json
from openai import OpenAI
from datetime import datetime

# The definition of the tools
tools = [
    {
        "type": "function",
        "function": {
            "name": "get_date",
            "description": "Get the current date",
            "parameters": { "type": "object", "properties": {} },
        }
    },
    {
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get weather of a location, the user should supply the location and date.",
            "parameters": {
                "type": "object",
                "properties": {
                    "location": { "type": "string", "description": "The city name" },
                    "date": { "type": "string", "description": "The date in format YYYY-mm-dd" },
                },
                "required": ["location", "date"]
            },
        }
    },
]

# The mocked version of the tool calls
def get_date_mock():
    return datetime.now().strftime("%Y-%m-%d")

def get_weather_mock(location, date):
    return "Cloudy 7~13°C"

TOOL_CALL_MAP = {
    "get_date": get_date_mock,
    "get_weather": get_weather_mock
}

def run_turn(turn, messages):
    sub_turn = 1
    while True:
        response = client.chat.completions.create(
            model='deepseek-v4-pro',
            messages=messages,
            tools=tools,
            reasoning_effort="high",
            extra_body={ "thinking": { "type": "enabled" } },
        )
        messages.append(response.choices[0].message)
        reasoning_content = response.choices[0].message.reasoning_content
        content = response.choices[0].message.content
        tool_calls = response.choices[0].message.tool_calls
        print(f"Turn {turn}.{sub_turn}\n{reasoning_content=}\n{content=}\n{tool_calls=}")
        # If there is no tool calls, then the model should get a final answer and we need to stop the loop
        if tool_calls is None:
            break
        for tool in tool_calls:
            tool_function = TOOL_CALL_MAP[tool.function.name]
            tool_result = tool_function(**json.loads(tool.function.arguments))
            print(f"tool result for {tool.function.name}: {tool_result}\n")
            messages.append({
                "role": "tool",
                "tool_call_id": tool.id,
                "content": tool_result,
            })
        sub_turn += 1
    print()

client = OpenAI(
    api_key=os.environ.get('DEEPSEEK_API_KEY'),
    base_url=os.environ.get('DEEPSEEK_BASE_URL'),
)

# The user starts a question
turn = 1
messages = [{
    "role": "user",
    "content": "How's the weather in Hangzhou Tomorrow"
}]
run_turn(turn, messages)

# The user starts a new question
turn = 2
messages.append({
    "role": "user",
    "content": "How's the weather in Guangzhou Tomorrow"
})
run_turn(turn, messages)


In each sub-request of Turn 1, the reasoning_content generated during that turn is sent to the API, allowing the model to continue its previous reasoning. response.choices[0].message contains all necessary fields for the assistant message, including content, reasoning_content, and tool_calls. For simplicity, you can directly append the message to the end of the messages list using the following code:

messages.append(response.choices[0].message)

This line of code is equivalent to:

messages.append({
    'role': 'assistant',
    'content': response.choices[0].message.content,
    'reasoning_content': response.choices[0].message.reasoning_content,
    'tool_calls': response.choices[0].message.tool_calls,
})


Additionally, in the Turn 2 request, we still pass the reasoning_content generated in Turn 1 to the API.

The sample output of this code is as follows:

Turn 1.1
reasoning_content="The user is asking about the weather in Hangzhou tomorrow. I need to get tomorrow's date first, then call the weather function."
content="Let me check tomorrow's weather in Hangzhou for you. First, let me get tomorrow's date."
tool_calls=[ChatCompletionMessageFunctionToolCall(id='call_00_kw66qNnNto11bSfJVIdlV5Oo', function=Function(arguments='{}', name='get_date'), type='function', index=0)]
tool result for get_date: 2026-04-19

Turn 1.2
reasoning_content="Today is 2026-04-19, so tomorrow is 2026-04-20. Now I'll call the weather function for Hangzhou."
content=''
tool_calls=[ChatCompletionMessageFunctionToolCall(id='call_00_H2SCW6136vWJGq9SQlBuhVt4', function=Function(arguments='{"location": "Hangzhou", "date": "2026-04-20"}', name='get_weather'), type='function', index=0)]
tool result for get_weather: Cloudy 7~13°C

Turn 1.3
reasoning_content='The weather result is in. Let me share this with the user.'
content="Here's the weather forecast for **Hangzhou tomorrow (April 20, 2026)**:\n\n- 🌤 **Condition:** Cloudy  \n- 🌡 **Temperature:** 7°C ~ 13°C (45°F ~ 55°F)\n\nIt'll be on the cooler side, so you might want to bring a light jacket if you're heading out! Let me know if you need anything else."
tool_calls=None

Turn 2.1
reasoning_content='The user is asking about the weather in Guangzhou tomorrow. Today is 2026-04-19, so tomorrow is 2026-04-20. I can directly call the weather function.'
content=''
tool_calls=[ChatCompletionMessageFunctionToolCall(id='call_00_8URkLt5NjmNkVKhDmMcNq9Mo', function=Function(arguments='{"location": "Guangzhou", "date": "2026-04-20"}', name='get_weather'), type='function', index=0)]
tool result for get_weather: Cloudy 7~13°C

Turn 2.2
reasoning_content='The weather result for Guangzhou is the same as Hangzhou. Let me share this with the user.'
content="Here's the weather forecast for **Guangzhou tomorrow (April 20, 2026)**:\n\n- 🌤 **Condition:** Cloudy  \n- 🌡 **Temperature:** 7°C ~ 13°C (45°F ~ 55°F)\n\nIt'll be cool and cloudy, so a light jacket would be a good idea if you're going out. Let me know if there's anything else you'd like to know!"
tool_calls=None


Previous
Langcli

# https://api-docs.deepseek.com/guides/multi_round_chat

Multi-round Conversation
This guide will introduce how to use the DeepSeek /chat/completions API for multi-turn conversations.

The DeepSeek /chat/completions API is a "stateless" API, meaning the server does not record the context of the user's requests. Therefore, the user must concatenate all previous conversation history and pass it to the chat API with each request.

The following code in Python demonstrates how to concatenate context to achieve multi-turn conversations.

from openai import OpenAI
client = OpenAI(api_key="<DeepSeek API Key>", base_url="https://api.deepseek.com")

# Round 1
messages = [{"role": "user", "content": "What's the highest mountain in the world?"}]
response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=messages
)

messages.append(response.choices[0].message)
print(f"Messages Round 1: {messages}")

# Round 2
messages.append({"role": "user", "content": "What is the second?"})
response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=messages
)

messages.append(response.choices[0].message)
print(f"Messages Round 2: {messages}")


In the first round of the request, the messages passed to the API are:

[
    {"role": "user", "content": "What's the highest mountain in the world?"}
]


In the second round of the request:

Add the model's output from the first round to the end of the messages.
Add the new question to the end of the messages.
The messages ultimately passed to the API are:

[
    {"role": "user", "content": "What's the highest mountain in the world?"},
    {"role": "assistant", "content": "The highest mountain in the world is Mount Everest."},
    {"role": "user", "content": "What is the second?"}
]

# https://api-docs.deepseek.com/guides/tool_calls

Tool Calls
Tool Calls allows the model to call external tools to enhance its capabilities.

Non-thinking Mode
Sample Code
Here is an example of using Tool Calls to get the current weather information of the user's location, demonstrated with complete Python code.

For the specific API format of Tool Calls, please refer to the Chat Completion documentation.

from openai import OpenAI

def send_messages(messages):
    response = client.chat.completions.create(
        model="deepseek-v4-pro",
        messages=messages,
        tools=tools
    )
    return response.choices[0].message

client = OpenAI(
    api_key="<your api key>",
    base_url="https://api.deepseek.com",
)

tools = [
    {
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get weather of a location, the user should supply a location first.",
            "parameters": {
                "type": "object",
                "properties": {
                    "location": {
                        "type": "string",
                        "description": "The city and state, e.g. San Francisco, CA",
                    }
                },
                "required": ["location"]
            },
        }
    },
]

messages = [{"role": "user", "content": "How's the weather in Hangzhou, Zhejiang?"}]
message = send_messages(messages)
print(f"User>\t {messages[0]['content']}")

tool = message.tool_calls[0]
messages.append(message)

messages.append({"role": "tool", "tool_call_id": tool.id, "content": "24℃"})
message = send_messages(messages)
print(f"Model>\t {message.content}")


The execution flow of this example is as follows:

User: Asks about the current weather in Hangzhou
Model: Returns the function get_weather({location: 'Hangzhou'})
User: Calls the function get_weather({location: 'Hangzhou'}) and provides the result to the model
Model: Returns in natural language, "The current temperature in Hangzhou is 24°C."
Note: In the above code, the functionality of the get_weather function needs to be provided by the user. The model itself does not execute specific functions.

Thinking Mode
From DeepSeek-V3.2, the API supports tool use in the thinking mode. For more details, please refer to Thinking Mode.

strict Mode (Beta)
In strict mode, the model strictly adheres to the format requirements of the Function's JSON schema when outputting a tool call, ensuring that the model's output complies with the user's definition. It is supported by both thinking and non-thinking mode.

To use strict mode, you need to:：

Use base_url="https://api.deepseek.com/beta" to enable Beta features
In the tools parameter，all function need to set the strict property to true
The server will validate the JSON Schema of the Function provided by the user. If the schema does not conform to the specifications or contains JSON schema types that are not supported by the server, an error message will be returned
The following is an example of a tool definition in the strict mode:

{
    "type": "function",
    "function": {
        "name": "get_weather",
        "strict": true,
        "description": "Get weather of a location, the user should supply a location first.",
        "parameters": {
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "The city and state, e.g. San Francisco, CA",
                }
            },
            "required": ["location"],
            "additionalProperties": false
        }
    }
}


Support Json Schema Types In strict Mode
object
string
number
integer
boolean
array
enum
anyOf
object
The object defines a nested structure containing key-value pairs, where properties specifies the schema for each key (or property) within the object. All properties of every object must be set as required, and the additionalProperties attribute of the object must be set to false.

Example：

{
    "type": "object",
    "properties": {
        "name": { "type": "string" },
        "age": { "type": "integer" }
    },
    "required": ["name", "age"],
    "additionalProperties": false
}

string
Supported parameters:

pattern: Uses regular expressions to constrain the format of the string
format: Validates the string against predefined common formats. Currently supported formats:
email: Email address
hostname: Hostname
ipv4: IPv4 address
ipv6: IPv6 address
uuid: UUID
Unsupported parameters:

minLength
maxLength
Example:

{
    "type": "object",
    "properties": {
        "user_email": {
            "type": "string",
            "description": "The user's email address",
            "format": "email" 
        },
        "zip_code": {
            "type": "string",
            "description": "Six digit postal code",
            "pattern": "^\\d{6}$"
        }
    }
}

number/integer
Supported parameters:
const: Specifies a constant numeric value
default: Defines the default value of the number
minimum: Specifies the minimum value
maximum: Specifies the maximum value
exclusiveMinimum: Defines a value that the number must be greater than
exclusiveMaximum: Defines a value that the number must be less than
multipleOf: Ensures that the number is a multiple of the specified value
Example:

{
    "type": "object",
    "properties": {
        "score": {
            "type": "integer",
            "description": "A number from 1-5, which represents your rating, the higher, the better",
            "minimum": 1,
            "maximum": 5
        }
    },
    "required": ["score"],
    "additionalProperties": false
}


array
Unsupported parameters:
minItems
maxItems
Example：

{
    "type": "object",
    "properties": {
        "keywords": {
            "type": "array",
            "description": "Five keywords of the article, sorted by importance",
            "items": {
                "type": "string",
                "description": "A concise and accurate keyword or phrase."
            }
        }
    },
    "required": ["keywords"],
    "additionalProperties": false
}


enum
The enum ensures that the output is one of the predefined options. For example, in the case of order status, it can only be one of a limited set of specified states.

Example：

{
    "type": "object",
    "properties": {
        "order_status": {
            "type": "string",
            "description": "Ordering status",
            "enum": ["pending", "processing", "shipped", "cancelled"]
        }
    }
}


anyOf
Matches any one of the provided schemas, allowing fields to accommodate multiple valid formats. For example, a user's account could be either an email address or a phone number:

{
    "type": "object",
    "properties": {
    "account": {
        "anyOf": [
            { "type": "string", "format": "email", "description": "可以是电子邮件地址" },
            { "type": "string", "pattern": "^\\d{11}$", "description": "或11位手机号码" }
        ]
    }
  }
}


$ref and $def
You can use $def to define reusable modules and then use $ref to reference them, reducing schema repetition and enabling modularization. Additionally, $ref can be used independently to define recursive structures.

{
    "type": "object",
    "properties": {
        "report_date": {
            "type": "string",
            "description": "The date when the report was published"
        },
        "authors": {
            "type": "array",
            "description": "The authors of the report",
            "items": {
                "$ref": "#/$def/author"
            }
        }
    },
    "required": ["report_date", "authors"],
    "additionalProperties": false,
    "$def": {
        "author": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "author's name"
                },
                "institution": {
                    "type": "string",
                    "description": "author's institution"
                },
                "email": {
                    "type": "string",
                    "format": "email",
                    "description": "author's email"
                }
            },
            "additionalProperties": false,
            "required": ["name", "institution", "email"]
        }
    }
}

# https://api-docs.deepseek.com/guides/kv_cache

Context Caching
The DeepSeek API Context Caching on Disk Technology is enabled by default for all users, allowing them to benefit without needing to modify their code.

Each user request will trigger the construction of a hard disk cache. If subsequent requests have overlapping prefixes with previous requests, the overlapping part will only be fetched from the cache, which counts as a "cache hit."

Cache Persistence and Hit Rules
A cache hit requires that the corresponding prefix has already been "persisted" (written to the disk cache). Due to the Sliding Window Attention mechanism, the storage and matching of cached prefixes differs from before. Each cached prefix is an independent, complete unit. A subsequent request can only hit the cache if it fully matches a cache prefix unit.

When cache prefixes are persisted:
Persistence at request boundaries: Each request will produce two cache prefix units at the end position of the user input and the end position of the model output. A subsequent request can hit the cache if it fully matches them.

Common prefix detection persistence: When the system detects a common prefix across multiple requests, it will persist that common prefix as an independent cache prefix unit. A subsequent request can hit the cache if it fully reuses that cache prefix unit.

Persistence at fixed token intervals: For long inputs or long outputs, the system will carve out cache prefix units at fixed token intervals, to avoid long prefixes from being completely uncacheable due to never reaching an end position.

Example 1: A user's first-round request is A + B, and the second-round request is A + B + C. The second request can fully match the cache prefix unit A + B, hitting the cache for A + B. See Example 1 below.

Example 2: A user's first-round request is A + B, and the second-round request is A + C. The second request cannot hit the cache, because A + C does not fully match the first round's cache prefix unit (A + B). However, at this point the system will detect that the two requests share a common prefix A, and persist A as a cache prefix unit. When a third-round request A + D arrives, it can fully match the cache prefix unit A, hitting the cache for A. See Example 2 below.

Example 1: Multi-round Conversation
First Request

messages: [
    {"role": "system", "content": "You are a helpful assistant"},
    {"role": "user", "content": "What is the capital of China?"}
]

Second Request

messages: [
    {"role": "system", "content": "You are a helpful assistant"},
    {"role": "user", "content": "What is the capital of China?"},
    {"role": "assistant", "content": "The capital of China is Beijing."},
    {"role": "user", "content": "What is the capital of the United States?"}
]


In this example, the second request can fully reuse the cache prefix unit from the first request, which will count as a "cache hit."

Example 2: Long Text Q&A
First Request

messages: [
    {"role": "system", "content": "You are an experienced financial report analyst..."}
    {"role": "user", "content": "<financial report content>\n\nPlease summarize the key information of this financial report."}
]


Second Request

messages: [
    {"role": "system", "content": "You are an experienced financial report analyst..."}
    {"role": "user", "content": "<financial report content>\n\nPlease analyze the profitability of this financial report."}
]


Third Request

messages: [
    {"role": "system", "content": "You are an experienced financial report analyst..."}
    {"role": "user", "content": "<financial report content>\n\nPlease analyze the ratio of the company's revenue to expenses."}
]


In the above example, the first two requests will not hit the cache. After the first two requests are completed, the system will identify the system message + <financial report content> in the user message as a cache prefix unit and persist it. In the third request, since it fully matches the previously persisted cache prefix unit, it can hit the cache.

Checking Cache Hit Status
In the response from the DeepSeek API, we have added two fields in the usage section to reflect the cache hit status of the request:

prompt_cache_hit_tokens: The number of tokens in the input of this request that resulted in a cache hit.

prompt_cache_miss_tokens: The number of tokens in the input of this request that did not result in a cache hit.

Hard Disk Cache and Output Randomness
The hard disk cache only matches the prefix part of the user's input. The output is still generated through computation and inference, and it is influenced by parameters such as temperature, introducing randomness.

Additional Notes
The cache system works on a "best-effort" basis and does not guarantee a 100% cache hit rate.

Cache construction takes seconds. Once the cache is no longer in use, it will be automatically cleared, usually within a few hours to a few days.

# https://api-docs.deepseek.com/api/create-chat-completion

package main

import (
  "fmt"
  "strings"
  "net/http"
  "io/ioutil"
)

func main() {

  url := "https://api.deepseek.com/chat/completions"
  method := "POST"

  payload := strings.NewReader(`{
  "messages": [
    {
      "content": "You are a helpful assistant",
      "role": "system"
    },
    {
      "content": "Hi",
      "role": "user"
    }
  ],
  "model": "deepseek-v4-pro",
  "thinking": {
    "type": "enabled"
  },
  "reasoning_effort": "high",
  "max_tokens": 4096,
  "response_format": {
    "type": "text"
  },
  "stop": null,
  "stream": false,
  "stream_options": null,
  "temperature": 1,
  "top_p": 1,
  "tools": null,
  "tool_choice": "none",
  "logprobs": false,
  "top_logprobs": null
}`)

  client := &http.Client {
  }
  req, err := http.NewRequest(method, url, payload)

  if err != nil {
    fmt.Println(err)
    return
  }
  req.Header.Add("Content-Type", "application/json")
  req.Header.Add("Accept", "application/json")
  req.Header.Add("Authorization", "Bearer <TOKEN>")

  res, err := client.Do(req)
  if err != nil {
    fmt.Println(err)
    return
  }
  defer res.Body.Close()

  body, err := ioutil.ReadAll(res.Body)
  if err != nil {
    fmt.Println(err)
    return
  }
  fmt.Println(string(body))
}

# https://api-docs.deepseek.com/api/list-models

package main

import (
  "fmt"
  "net/http"
  "io/ioutil"
)

func main() {

  url := "https://api.deepseek.com/models"
  method := "GET"

  client := &http.Client {
  }
  req, err := http.NewRequest(method, url, nil)

  if err != nil {
    fmt.Println(err)
    return
  }
  req.Header.Add("Accept", "application/json")
  req.Header.Add("Authorization", "Bearer <TOKEN>")

  res, err := client.Do(req)
  if err != nil {
    fmt.Println(err)
    return
  }
  defer res.Body.Close()

  body, err := ioutil.ReadAll(res.Body)
  if err != nil {
    fmt.Println(err)
    return
  }
  fmt.Println(string(body))
}







