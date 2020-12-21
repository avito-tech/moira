package slack

import "testing"

func TestCommandsExtraction(t *testing.T) {
	type TestCase struct {
		Input                          string
		ExpectedMessageWithoutCommands string
		ExpectedJoinedCommands         string
	}

	testCases := []TestCase{
		{
			Input:                          "Some text here.\n_include_top_error_\nMore text.\n_get_screen(some-param, 20)_",
			ExpectedMessageWithoutCommands: "Some text here.\nMore text.",
			ExpectedJoinedCommands:         "_include_top_error_\n_get_screen(some-param, 20)_",
		},

		{
			Input:                          "Some text here.\n_include_top_error_",
			ExpectedMessageWithoutCommands: "Some text here.",
			ExpectedJoinedCommands:         "_include_top_error_",
		},
		{
			Input:                          "Some text here.\n_include_top_error_\n",
			ExpectedMessageWithoutCommands: "Some text here.",
			ExpectedJoinedCommands:         "_include_top_error_",
		},

		{
			Input:                          "Some text here.\n_get_screen(some-param, 20)_",
			ExpectedMessageWithoutCommands: "Some text here.",
			ExpectedJoinedCommands:         "_get_screen(some-param, 20)_",
		},
		{
			Input:                          "Some text here.\n_get_screen(some-param, 20)_\n",
			ExpectedMessageWithoutCommands: "Some text here.",
			ExpectedJoinedCommands:         "_get_screen(some-param, 20)_",
		},

		{
			Input:                          "Some text here.\nMore text.\n_deploy_status_all_ _include_top_error_",
			ExpectedMessageWithoutCommands: "Some text here.\nMore text.",
			ExpectedJoinedCommands:         "_deploy_status_all_ _include_top_error_",
		},

		{
			Input:                          "Some text here.\n_get_screen(some-param, 20)_\n```\nwoohoo I'm code\n```",
			ExpectedMessageWithoutCommands: "Some text here.\n```\nwoohoo I'm code\n```",
			ExpectedJoinedCommands:         "_get_screen(some-param, 20)_",
		},

		{
			Input:                          "_get_screen(some-param, 20)_",
			ExpectedMessageWithoutCommands: "",
			ExpectedJoinedCommands:         "_get_screen(some-param, 20)_",
		},
		{
			Input:                          "Some text.\n\nSome more text",
			ExpectedMessageWithoutCommands: "Some text.\n\nSome more text",
			ExpectedJoinedCommands:         "",
		},
	}

	for i, testCase := range testCases {
		mWC := removeAvitoErrbotCommands(testCase.Input)
		jC := extractAvitoErrbotCommands(testCase.Input)
		if mWC != testCase.ExpectedMessageWithoutCommands {
			t.Errorf("Test case %d, messageWithoutCommands = %q, expected %q", i, mWC, testCase.ExpectedMessageWithoutCommands)
		}
		if jC != testCase.ExpectedJoinedCommands {
			t.Errorf("Test case %d, joinedCommands = %q, expected %q", i, jC, testCase.ExpectedJoinedCommands)
		}
	}
}
